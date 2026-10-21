// Package adminhdl exposes the admin section (ADR 0029 steps 1.3 and 2):
// the users with their characters' names, the pending invites, inviting and
// cancelling an invite, and an admin's change to a user (disable, re-enable,
// make or unmake an admin). The router mounts it behind mw.AdminOnly: a
// player never reaches these handlers.
package adminhdl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ceffo/critical/api/internal/domain"
	"github.com/ceffo/critical/api/internal/domain/service/authsvc"
	"github.com/ceffo/critical/api/internal/render"
	mw "github.com/ceffo/critical/api/internal/rest/middleware"
)

// Router is the chi subset we need.
type Router interface {
	Get(pattern string, h http.HandlerFunc)
	Post(pattern string, h http.HandlerFunc)
	Patch(pattern string, h http.HandlerFunc)
	Delete(pattern string, h http.HandlerFunc)
}

var _ Router = (*chi.Mux)(nil)

// Service is what the handlers call (authsvc.SVC satisfies it).
type Service interface {
	ListUsers(ctx context.Context) ([]authsvc.UserSummary, error)
	ListInvites(ctx context.Context) ([]authsvc.Invite, error)
	CreateInvite(ctx context.Context, by uuid.UUID, email string, lang authsvc.Language) (*authsvc.Invite, authsvc.MailOutcome, error)
	ResendInvite(ctx context.Context, id uuid.UUID) (*authsvc.Invite, authsvc.MailOutcome, error)
	CancelInvite(ctx context.Context, id uuid.UUID) error
	ChangeUser(ctx context.Context, id uuid.UUID, change authsvc.UserChange) (*authsvc.UserSummary, error)
}

// RegisterRoutes wires the admin endpoints onto r, which the caller guards
// with mw.AdminOnly.
func RegisterRoutes(r Router, svc Service) {
	r.Get("/api/admin/users", render.Handle(ListUsers(svc)))
	r.Patch("/api/admin/users/{id}", render.Handle(ChangeUser(svc)))
	r.Get("/api/admin/invites", render.Handle(ListInvites(svc)))
	r.Post("/api/admin/invites", render.Handle(CreateInvite(svc)))
	r.Post("/api/admin/invites/{id}/resend", render.Handle(ResendInvite(svc)))
	r.Delete("/api/admin/invites/{id}", render.Handle(CancelInvite(svc)))
}

// UserDTO is a user in the admin section: never their sheets, only their
// characters' names (step 1.3). LastLogin is null before the first login.
type UserDTO struct {
	ID         string   `json:"id"`
	Email      string   `json:"email"`
	Role       string   `json:"role"`
	Root       bool     `json:"root"`
	Disabled   bool     `json:"disabled"`
	LastLogin  *string  `json:"lastLogin"`
	CreatedAt  string   `json:"createdAt"`
	Characters []string `json:"characters"`
}

// UsersDTO is GET /api/admin/users.
type UsersDTO struct {
	Users []UserDTO `json:"users"`
}

// InviteDTO is a pending invite: the email, who invited (a user id), when,
// and the language of its mails (ADR 0029 step 2.6).
type InviteDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	InvitedBy string `json:"invitedBy"`
	CreatedAt string `json:"createdAt"`
	Language  string `json:"language"`
}

// InviteSentDTO is the answer of creating or resending an invite: the invite,
// and what became of its mail (step 2.8): "sent", "failed" or
// "not_configured". The web words it.
type InviteSentDTO struct {
	InviteDTO
	Mail string `json:"mail"`
}

// InvitesDTO is GET /api/admin/invites.
type InvitesDTO struct {
	Invites []InviteDTO `json:"invites"`
}

// stamp is the API's time format: UTC, whole seconds.
func stamp(t time.Time) string { return t.UTC().Truncate(time.Second).Format(time.RFC3339) }

func userDTO(u authsvc.UserSummary) UserDTO {
	d := UserDTO{
		ID: u.ID.String(), Email: u.Email, Role: string(u.Role), Root: u.Root, Disabled: u.Disabled,
		CreatedAt: stamp(u.CreatedAt), Characters: u.Characters,
	}
	if d.Characters == nil {
		d.Characters = []string{}
	}
	if u.LastLogin != nil {
		s := stamp(*u.LastLogin)
		d.LastLogin = &s
	}
	return d
}

func inviteDTO(i authsvc.Invite) InviteDTO {
	return InviteDTO{ID: i.ID.String(), Email: i.Email, InvitedBy: i.InvitedBy.String(), CreatedAt: stamp(i.CreatedAt), Language: string(i.Language)}
}

// idParam is the {id} of the path, a UUID.
func idParam(r *http.Request) (uuid.UUID, error) {
	raw := chi.URLParam(r, "id")
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, domain.FieldErrorf("id", "%q is not an id", raw)
	}
	return id, nil
}

// ListUsers is GET /api/admin/users.
func ListUsers(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		users, err := svc.ListUsers(r.Context())
		if err != nil {
			return err
		}
		out := UsersDTO{Users: make([]UserDTO, 0, len(users))}
		for _, u := range users {
			out.Users = append(out.Users, userDTO(u))
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// changeBody is PATCH /api/admin/users/{id}: each field optional.
type changeBody struct {
	Disabled *bool   `json:"disabled"`
	Role     *string `json:"role"`
}

// ChangeUser is PATCH /api/admin/users/{id} {disabled?, role?}: the user as
// changed.
func ChangeUser(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := idParam(r)
		if err != nil {
			return err
		}
		var body changeBody
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		if body.Disabled == nil && body.Role == nil {
			return fmt.Errorf("%w: nothing to change: send disabled or role", domain.ErrInvalidInput)
		}
		change := authsvc.UserChange{Disabled: body.Disabled}
		if body.Role != nil {
			role := authsvc.Role(*body.Role)
			change.Role = &role
		}
		u, err := svc.ChangeUser(r.Context(), id, change)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, userDTO(*u))
		return nil
	}
}

// ListInvites is GET /api/admin/invites: the pending invites.
func ListInvites(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		invites, err := svc.ListInvites(r.Context())
		if err != nil {
			return err
		}
		out := InvitesDTO{Invites: make([]InviteDTO, 0, len(invites))}
		for _, i := range invites {
			out.Invites = append(out.Invites, inviteDTO(i))
		}
		render.OK(r.Context(), w, out)
		return nil
	}
}

// CreateInvite is POST /api/admin/invites {email, language?}: 201 with the
// invite and the outcome of its mail, invited by the request's user.
// language is en or fr; absent, en.
func CreateInvite(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		p := mw.PrincipalFrom(r.Context())
		if p == nil {
			return authsvc.ErrUnauthenticated
		}
		var body struct {
			Email    string `json:"email"`
			Language string `json:"language"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return render.BadRequestError("invalid request body")
		}
		inv, outcome, err := svc.CreateInvite(r.Context(), p.User.ID, body.Email, authsvc.Language(body.Language))
		if err != nil {
			return err
		}
		render.JSON(r.Context(), w, http.StatusCreated, InviteSentDTO{InviteDTO: inviteDTO(*inv), Mail: string(outcome)})
		return nil
	}
}

// ResendInvite is POST /api/admin/invites/{id}/resend: the invite and the
// outcome of its mail (step 2.7). A cancelled or used invite is a 409.
func ResendInvite(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := idParam(r)
		if err != nil {
			return err
		}
		inv, outcome, err := svc.ResendInvite(r.Context(), id)
		if err != nil {
			return err
		}
		render.OK(r.Context(), w, InviteSentDTO{InviteDTO: inviteDTO(*inv), Mail: string(outcome)})
		return nil
	}
}

// CancelInvite is DELETE /api/admin/invites/{id}: the invite is cancelled,
// never deleted (step 2.3); the answer is the pending invites left.
func CancelInvite(svc Service) render.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		id, err := idParam(r)
		if err != nil {
			return err
		}
		if err := svc.CancelInvite(r.Context(), id); err != nil {
			return err
		}
		return ListInvites(svc)(w, r)
	}
}
