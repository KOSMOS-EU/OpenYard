// Package apply creates an Aktenplan folder structure in OpenCloud via CS3.
package apply

import (
	"context"
	"fmt"
	"io"
	"strings"

	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	gateway "github.com/cs3org/go-cs3apis/cs3/gateway/v1beta1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"github.com/kosmos-eu/openyard/aktenplan-setup/pkg/schema"
)

// Options configures the apply operation.
type Options struct {
	GatewayAddr string
	Username    string
	Password    string
	SpaceName   string // Override space name from YAML
	BasePath    string // Path prefix in the space (e.g., "/Aktenplan")
	DryRun      bool
	Output      io.Writer
}

// Applier holds the connection and state for applying an Aktenplan.
type Applier struct {
	opts    Options
	conn    *grpc.ClientConn
	gw      gateway.GatewayAPIClient
	token   string
	created int
	skipped int
}

// New creates a new Applier with a CS3 gateway connection.
func New(opts Options) (*Applier, error) {
	conn, err := grpc.NewClient(opts.GatewayAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to gateway %s: %w", opts.GatewayAddr, err)
	}
	return &Applier{
		opts: opts,
		conn: conn,
		gw:   gateway.NewGatewayAPIClient(conn),
	}, nil
}

// Close releases the gateway connection.
func (a *Applier) Close() error {
	return a.conn.Close()
}

// Run authenticates and applies the Aktenplan tree.
func (a *Applier) Run(ctx context.Context, ap *schema.Aktenplan) error {
	// In dry-run mode, skip authentication
	if !a.opts.DryRun {
		if err := a.authenticate(ctx); err != nil {
			return err
		}
	}

	basePath := a.opts.BasePath
	if basePath == "" {
		basePath = "/"
		if ap.Aktenplan.Space != nil && ap.Aktenplan.Space.Name != "" {
			basePath = "/" + ap.Aktenplan.Space.Name
		}
	}

	total := schema.CountKnoten(ap.Aktenplan.Knoten)
	fmt.Fprintf(a.opts.Output, "Aktenplan: %d Knoten, Basis-Pfad: %s\n", total, basePath)

	if a.opts.DryRun {
		fmt.Fprintln(a.opts.Output, "[DRY-RUN] Keine Änderungen werden durchgeführt.")
	}

	// Ensure base path exists
	if !a.opts.DryRun {
		if err := a.ensureContainer(ctx, basePath); err != nil {
			return fmt.Errorf("base path %s: %w", basePath, err)
		}
	}

	// Walk and create
	schema.Walk(ap.Aktenplan.Knoten, basePath, func(path string, k *schema.Knoten, depth int) {
		indent := strings.Repeat("  ", depth)

		// Build display label
		typeTag := ""
		if k.Typ != "" {
			typeTag = " [" + k.Typ + "]"
		}
		protTag := ""
		if k.Immutable {
			protTag = " (protected)"
		}

		if a.opts.DryRun {
			fmt.Fprintf(a.opts.Output, "%s[DRY] mkdir %s%s%s\n", indent, path, typeTag, protTag)
			for _, r := range k.Rechte {
				fmt.Fprintf(a.opts.Output, "%s  → recht: %s=%s\n", indent, r.Rolle, r.Wirkung)
			}
			return
		}

		err := a.ensureContainer(ctx, path)
		if err != nil {
			fmt.Fprintf(a.opts.Output, "%sERROR %s: %v\n", indent, path, err)
			return
		}

		// Build metadata
		md := map[string]string{
			"oy.fileReference": k.Kennung,
		}
		if k.Typ != "" {
			md["_type_"+k.Typ] = "true"
		}
		if k.Immutable {
			md["oy.protected"] = "true"
		}

		a.setMetadata(ctx, path, md)

		if depth <= 2 || (a.created+a.skipped)%50 == 0 {
			fmt.Fprintf(a.opts.Output, "%sOK %s%s%s\n", indent, path, typeTag, protTag)
		}
	})

	fmt.Fprintf(a.opts.Output, "\nFertig: %d erstellt, %d bereits vorhanden\n", a.created, a.skipped)
	return nil
}

func (a *Applier) authenticate(ctx context.Context) error {
	res, err := a.gw.Authenticate(ctx, &gateway.AuthenticateRequest{
		Type:         "basic",
		ClientId:     a.opts.Username,
		ClientSecret: a.opts.Password,
	})
	if err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return fmt.Errorf("authenticate: %s", res.Status.Message)
	}
	a.token = res.Token
	return nil
}

func (a *Applier) ctx(parent context.Context) context.Context {
	return metadata.AppendToOutgoingContext(parent, "x-access-token", a.token)
}

func (a *Applier) ensureContainer(ctx context.Context, path string) error {
	ctx = a.ctx(ctx)

	// Check if exists
	statRes, err := a.gw.Stat(ctx, &provider.StatRequest{
		Ref: &provider.Reference{Path: path},
	})
	if err == nil && statRes.Status.Code == rpc.Code_CODE_OK {
		a.skipped++
		return nil // already exists
	}

	// Create
	res, err := a.gw.CreateContainer(ctx, &provider.CreateContainerRequest{
		Ref: &provider.Reference{Path: path},
	})
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	if res.Status.Code != rpc.Code_CODE_OK && res.Status.Code != rpc.Code_CODE_ALREADY_EXISTS {
		return fmt.Errorf("create %s: %s", path, res.Status.Message)
	}
	a.created++
	return nil
}

func (a *Applier) setMetadata(ctx context.Context, path string, md map[string]string) {
	ctx = a.ctx(ctx)
	_, _ = a.gw.SetArbitraryMetadata(ctx, &provider.SetArbitraryMetadataRequest{
		Ref: &provider.Reference{Path: path},
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: md},
	})
}
