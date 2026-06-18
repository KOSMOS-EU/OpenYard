// Package apply creates an Aktenplan folder structure in OpenCloud via CS3.
package apply

import (
	"context"
	"fmt"
	"io"
	"strings"

	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
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
	user    *userpb.User
	spaceRoot *provider.ResourceId // root ResourceId of the target space
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

	spaceName := a.opts.SpaceName
	if spaceName == "" && ap.Aktenplan.Space != nil {
		spaceName = ap.Aktenplan.Space.Name
	}
	if spaceName == "" {
		return fmt.Errorf("space name required (--space-name or from YAML)")
	}

	total := schema.CountKnoten(ap.Aktenplan.Knoten)
	fmt.Fprintf(a.opts.Output, "Space: %s, Knoten: %d\n", spaceName, total)

	if a.opts.DryRun {
		fmt.Fprintln(a.opts.Output, "[DRY-RUN] Keine Änderungen werden durchgeführt.")
		fmt.Fprintf(a.opts.Output, "[DRY] create space %q\n", spaceName)
	}

	// Create or find the space
	if !a.opts.DryRun {
		if err := a.ensureSpace(ctx, spaceName); err != nil {
			return fmt.Errorf("space %q: %w", spaceName, err)
		}
		fmt.Fprintf(a.opts.Output, "Space ready: %s\n", spaceName)
	}

	// Walk and create folders inside the space
	schema.Walk(ap.Aktenplan.Knoten, "", func(relPath string, k *schema.Knoten, depth int) {
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

		// Display name for the folder segment
		folderName := k.Kennung
		if k.Name != "" {
			folderName = k.Kennung + " " + k.Name
		}

		if a.opts.DryRun {
			fmt.Fprintf(a.opts.Output, "%s[DRY] mkdir %s%s%s\n", indent, folderName, typeTag, protTag)
			return
		}

		err := a.ensureContainer(ctx, relPath)
		if err != nil {
			fmt.Fprintf(a.opts.Output, "%sERROR %s: %v\n", indent, folderName, err)
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

		a.setMetadata(ctx, relPath, md)

		if depth <= 2 || (a.created+a.skipped)%50 == 0 {
			fmt.Fprintf(a.opts.Output, "%sOK %s%s%s\n", indent, folderName, typeTag, protTag)
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
	a.user = res.User
	return nil
}

func (a *Applier) ctx(parent context.Context) context.Context {
	return metadata.AppendToOutgoingContext(parent, "x-access-token", a.token)
}

// ensureSpace creates a storage space or finds an existing one by name.
func (a *Applier) ensureSpace(ctx context.Context, spaceName string) error {
	ctx = a.ctx(ctx)

	// List existing spaces and check if one matches
	listRes, err := a.gw.ListStorageSpaces(ctx, &provider.ListStorageSpacesRequest{})
	if err == nil && listRes.Status.Code == rpc.Code_CODE_OK {
		for _, space := range listRes.StorageSpaces {
			if space.Name == spaceName {
				a.spaceRoot = space.Root
				fmt.Fprintf(a.opts.Output, "  Space exists: %s\n", spaceName)
				return nil
			}
		}
	}

	// Create new space
	createRes, err := a.gw.CreateStorageSpace(ctx, &provider.CreateStorageSpaceRequest{
		Type:  "project",
		Name:  spaceName,
		Owner: a.user,
	})
	if err != nil {
		return fmt.Errorf("create space: %w", err)
	}
	if createRes.Status.Code != rpc.Code_CODE_OK {
		return fmt.Errorf("create space: %s", createRes.Status.Message)
	}
	if createRes.StorageSpace != nil && createRes.StorageSpace.Root != nil {
		a.spaceRoot = createRes.StorageSpace.Root
	}
	fmt.Fprintf(a.opts.Output, "  Space created: %s\n", spaceName)
	return nil
}

// ref builds a CS3 Reference relative to the space root.
func (a *Applier) ref(relPath string) *provider.Reference {
	if a.spaceRoot != nil {
		p := "."
		if relPath != "" && relPath != "/" {
			p = "." + relPath
		}
		return &provider.Reference{ResourceId: a.spaceRoot, Path: p}
	}
	// Fallback to absolute path (shouldn't happen in normal flow)
	return &provider.Reference{Path: relPath}
}

func (a *Applier) ensureContainer(ctx context.Context, relPath string) error {
	ctx = a.ctx(ctx)

	ref := a.ref(relPath)

	// Check if exists
	statRes, err := a.gw.Stat(ctx, &provider.StatRequest{Ref: ref})
	if err == nil && statRes.Status.Code == rpc.Code_CODE_OK {
		a.skipped++
		return nil
	}

	// Create
	res, err := a.gw.CreateContainer(ctx, &provider.CreateContainerRequest{Ref: ref})
	if err != nil {
		return fmt.Errorf("create %s: %w", relPath, err)
	}
	if res.Status.Code != rpc.Code_CODE_OK && res.Status.Code != rpc.Code_CODE_ALREADY_EXISTS {
		return fmt.Errorf("create %s: %s", relPath, res.Status.Message)
	}
	a.created++
	return nil
}

func (a *Applier) setMetadata(ctx context.Context, relPath string, md map[string]string) {
	ctx = a.ctx(ctx)
	_, _ = a.gw.SetArbitraryMetadata(ctx, &provider.SetArbitraryMetadataRequest{
		Ref:               a.ref(relPath),
		ArbitraryMetadata: &provider.ArbitraryMetadata{Metadata: md},
	})
}
