package cmis

import (
	"net/http"
	"strings"

	grouppb "github.com/cs3org/go-cs3apis/cs3/identity/group/v1beta1"
	userpb "github.com/cs3org/go-cs3apis/cs3/identity/user/v1beta1"
	rpc "github.com/cs3org/go-cs3apis/cs3/rpc/v1beta1"
	collaboration "github.com/cs3org/go-cs3apis/cs3/sharing/collaboration/v1beta1"
	provider "github.com/cs3org/go-cs3apis/cs3/storage/provider/v1beta1"
	"github.com/rs/zerolog/log"
)

// CMIS ACL types

type ACL struct {
	ACEs     []ACE  `json:"aces"`
	IsExact  bool   `json:"isExact"`
}

type ACE struct {
	Principal  ACEPrincipal `json:"principal"`
	Permissions []string    `json:"permissions"`
	IsDirect   bool         `json:"isDirect"`
}

type ACEPrincipal struct {
	PrincipalId string `json:"principalId"`
}

// CMIS permission constants mapped from CS3 permissions.
const (
	PermCMISRead  = "cmis:read"
	PermCMISWrite = "cmis:write"
	PermCMISAll   = "cmis:all"
)

// GetACL handles cmisselector=acl — returns the ACL for an object.
func (h *Handler) GetACL(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Stat the object to get owner + permissions
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}

	aces := []ACE{}

	// Owner ACE (always has cmis:all)
	if statRes.Info.Owner != nil {
		aces = append(aces, ACE{
			Principal:   ACEPrincipal{PrincipalId: statRes.Info.Owner.OpaqueId},
			Permissions: []string{PermCMISAll},
			IsDirect:    true,
		})
	}

	// List shares on this resource → map to ACEs
	shares := h.listSharesForResource(r, statRes.Info.Id)
	for _, share := range shares {
		ace := shareToACE(share)
		if ace != nil {
			aces = append(aces, *ace)
		}
	}

	writeJSON(w, 200, ACL{
		ACEs:    aces,
		IsExact: true,
	})
}

// ApplyACL handles cmisaction=applyACL — adds/removes ACEs.
// Expects form fields:
//   addACEPrincipal[0]=userId  addACEPermission[0]=cmis:write
//   removeACEPrincipal[0]=userId
func (h *Handler) ApplyACL(w http.ResponseWriter, r *http.Request, ref *provider.Reference) {
	// Stat to get resource ID
	statRes, err := h.gw.Gateway.Stat(r.Context(), &provider.StatRequest{Ref: ref})
	if err != nil || statRes.Status.Code != rpc.Code_CODE_OK {
		writeError(w, 404, "objectNotFound", "Object not found")
		return
	}

	// Process removals first
	for i := 0; i < 50; i++ {
		principal := r.FormValue(formKey("removeACEPrincipal", i))
		if principal == "" {
			break
		}
		h.removeShareByPrincipal(r, statRes.Info.Id, principal)
	}

	// Process additions
	for i := 0; i < 50; i++ {
		principal := r.FormValue(formKey("addACEPrincipal", i))
		if principal == "" {
			break
		}
		permission := r.FormValue(formKey("addACEPermission", i))
		if permission == "" {
			permission = PermCMISRead
		}
		h.createShareForPrincipal(r, statRes.Info.Id, principal, permission)
	}

	// Return updated ACL
	h.GetACL(w, r, ref)
}

// --- Share ↔ ACL mapping ---

func (h *Handler) listSharesForResource(r *http.Request, rid *provider.ResourceId) []*collaboration.Share {
	res, err := h.gw.Gateway.ListShares(r.Context(), &collaboration.ListSharesRequest{
		Filters: []*collaboration.Filter{
			{
				Type: collaboration.Filter_TYPE_RESOURCE_ID,
				Term: &collaboration.Filter_ResourceId{ResourceId: rid},
			},
		},
	})
	if err != nil {
		log.Debug().Err(err).Msg("cmis: list shares failed")
		return nil
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		return nil
	}
	return res.Shares
}

func shareToACE(share *collaboration.Share) *ACE {
	if share.Grantee == nil {
		return nil
	}

	principalID := ""
	switch g := share.Grantee.Id.(type) {
	case *provider.Grantee_UserId:
		principalID = g.UserId.OpaqueId
	case *provider.Grantee_GroupId:
		principalID = "group:" + g.GroupId.OpaqueId
	default:
		return nil
	}

	perms := sharePermsToCMIS(share.Permissions)

	return &ACE{
		Principal:   ACEPrincipal{PrincipalId: principalID},
		Permissions: perms,
		IsDirect:    true,
	}
}

func sharePermsToCMIS(sp *collaboration.SharePermissions) []string {
	if sp == nil || sp.Permissions == nil {
		return []string{PermCMISRead}
	}
	p := sp.Permissions

	// If all write+delete+share permissions → cmis:all
	if p.InitiateFileUpload && p.Delete && p.AddGrant {
		return []string{PermCMISAll}
	}
	// If upload/create → cmis:write
	if p.InitiateFileUpload || p.CreateContainer {
		return []string{PermCMISRead, PermCMISWrite}
	}
	return []string{PermCMISRead}
}

func cmisPermToCS3(perm string) *provider.ResourcePermissions {
	switch perm {
	case PermCMISAll:
		return &provider.ResourcePermissions{
			Stat:                 true,
			ListContainer:        true,
			InitiateFileDownload: true,
			InitiateFileUpload:   true,
			CreateContainer:      true,
			Delete:               true,
			Move:                 true,
			AddGrant:             true,
			RemoveGrant:          true,
			UpdateGrant:          true,
		}
	case PermCMISWrite:
		return &provider.ResourcePermissions{
			Stat:                 true,
			ListContainer:        true,
			InitiateFileDownload: true,
			InitiateFileUpload:   true,
			CreateContainer:      true,
			Move:                 true,
		}
	default: // cmis:read
		return &provider.ResourcePermissions{
			Stat:                 true,
			ListContainer:        true,
			InitiateFileDownload: true,
		}
	}
}

func (h *Handler) createShareForPrincipal(r *http.Request, rid *provider.ResourceId, principal, perm string) {
	grantee := &provider.Grantee{}

	if strings.HasPrefix(principal, "group:") {
		grantee.Type = provider.GranteeType_GRANTEE_TYPE_GROUP
		grantee.Id = &provider.Grantee_GroupId{
			GroupId: &grouppb.GroupId{OpaqueId: strings.TrimPrefix(principal, "group:")},
		}
	} else {
		grantee.Type = provider.GranteeType_GRANTEE_TYPE_USER
		grantee.Id = &provider.Grantee_UserId{
			UserId: &userpb.UserId{OpaqueId: principal},
		}
	}

	perms := cmisPermToCS3(perm)

	res, err := h.gw.Gateway.CreateShare(r.Context(), &collaboration.CreateShareRequest{
		ResourceInfo: &provider.ResourceInfo{Id: rid},
		Grant: &collaboration.ShareGrant{
			Grantee:     grantee,
			Permissions: &collaboration.SharePermissions{Permissions: perms},
		},
	})
	if err != nil {
		log.Error().Err(err).Str("principal", principal).Msg("cmis: create share failed")
		return
	}
	if res.Status.Code != rpc.Code_CODE_OK {
		log.Warn().Str("msg", res.Status.Message).Str("principal", principal).Msg("cmis: create share rejected")
	}
}

func (h *Handler) removeShareByPrincipal(r *http.Request, rid *provider.ResourceId, principal string) {
	shares := h.listSharesForResource(r, rid)
	for _, share := range shares {
		if share.Grantee == nil {
			continue
		}
		pid := ""
		switch g := share.Grantee.Id.(type) {
		case *provider.Grantee_UserId:
			pid = g.UserId.OpaqueId
		case *provider.Grantee_GroupId:
			pid = "group:" + g.GroupId.OpaqueId
		}
		if pid == principal {
			res, err := h.gw.Gateway.RemoveShare(r.Context(), &collaboration.RemoveShareRequest{
				Ref: &collaboration.ShareReference{
					Spec: &collaboration.ShareReference_Id{Id: share.Id},
				},
			})
			if err != nil {
				log.Error().Err(err).Str("principal", principal).Msg("cmis: remove share failed")
			} else if res.Status.Code != rpc.Code_CODE_OK {
				log.Warn().Str("msg", res.Status.Message).Msg("cmis: remove share rejected")
			}
		}
	}
}

func formKey(prefix string, idx int) string {
	return prefix + "[" + strings.Repeat("", 0) + itoa2(idx) + "]"
}

func itoa2(i int) string {
	if i < 10 {
		return string(rune('0' + i))
	}
	return itoa2(i/10) + string(rune('0'+i%10))
}
