package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/rohit-bagade/notifyx/internal/domain"
	"github.com/rohit-bagade/notifyx/internal/repository/postgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newTemplateHandler(repo *mockTemplateRepo) *TemplateHandler {
	return NewTemplateHandler(&TemplateService{Templates: repo}, newTestLogger())
}

func TestTemplateHandler_Create_Success(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	want := &domain.NotificationTemplate{ID: uuid.New(), TenantID: tenant.ID, Name: "welcome", Channel: domain.ChannelEmail, Subject: "hi", Body: "body"}
	repo.On("Create", mock.Anything, tenant.ID, "welcome", domain.ChannelEmail, "hi", "body").Return(want, nil)

	c, rec := newCtx(http.MethodPost, "/api/v1/templates", `{"name":"welcome","channel":"email","subject":"hi","body":"body"}`, tenant)
	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)
	repo.AssertExpectations(t)
}

func TestTemplateHandler_Create_ValidationError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/templates", `{"name":"","channel":"email","subject":"hi","body":"body"}`, tenant)
	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	repo.AssertNotCalled(t, "Create")
}

func TestTemplateHandler_Create_BadBody(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	c, rec := newCtx(http.MethodPost, "/api/v1/templates", `{invalid json`, tenant)
	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTemplateHandler_Create_AlreadyExists(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	repo.On("Create", mock.Anything, tenant.ID, "welcome", domain.ChannelEmail, "hi", "body").Return(nil, postgres.ErrAlreadyExists)

	c, rec := newCtx(http.MethodPost, "/api/v1/templates", `{"name":"welcome","channel":"email","subject":"hi","body":"body"}`, tenant)
	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestTemplateHandler_Create_RepoError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	repo.On("Create", mock.Anything, tenant.ID, "welcome", domain.ChannelEmail, "hi", "body").Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodPost, "/api/v1/templates", `{"name":"welcome","channel":"email","subject":"hi","body":"body"}`, tenant)
	err := h.Create(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTemplateHandler_List_Success(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	want := []*domain.NotificationTemplate{{ID: uuid.New(), TenantID: tenant.ID}}
	repo.On("ListByTenant", mock.Anything, tenant.ID).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/templates", "", tenant)
	err := h.List(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTemplateHandler_List_RepoError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	repo.On("ListByTenant", mock.Anything, tenant.ID).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/templates", "", tenant)
	err := h.List(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTemplateHandler_Get_Success(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	want := &domain.NotificationTemplate{ID: id, TenantID: tenant.ID}
	repo.On("GetByID", mock.Anything, tenant.ID, id).Return(want, nil)

	c, rec := newCtx(http.MethodGet, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTemplateHandler_Get_BadID(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	c, rec := newCtx(http.MethodGet, "/api/v1/templates/not-a-uuid", "", tenant)
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTemplateHandler_Get_NotFound(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("GetByID", mock.Anything, tenant.ID, id).Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodGet, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTemplateHandler_Get_RepoError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("GetByID", mock.Anything, tenant.ID, id).Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodGet, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Get(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTemplateHandler_Update_Success(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	want := &domain.NotificationTemplate{ID: id, TenantID: tenant.ID, Name: "updated"}
	repo.On("Update", mock.Anything, tenant.ID, id, "updated", domain.ChannelEmail, "subj", "body").Return(want, nil)

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/"+id.String(), `{"name":"updated","channel":"email","subject":"subj","body":"body"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestTemplateHandler_Update_BadID(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/bad", `{"name":"x","channel":"email","subject":"s","body":"b"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTemplateHandler_Update_ValidationError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/"+id.String(), `{"name":"","channel":"email","subject":"s","body":"b"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestTemplateHandler_Update_NotFound(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Update", mock.Anything, tenant.ID, id, "updated", domain.ChannelEmail, "subj", "body").Return(nil, postgres.ErrNotFound)

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/"+id.String(), `{"name":"updated","channel":"email","subject":"subj","body":"body"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTemplateHandler_Update_AlreadyExists(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Update", mock.Anything, tenant.ID, id, "updated", domain.ChannelEmail, "subj", "body").Return(nil, postgres.ErrAlreadyExists)

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/"+id.String(), `{"name":"updated","channel":"email","subject":"subj","body":"body"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestTemplateHandler_Update_RepoError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Update", mock.Anything, tenant.ID, id, "updated", domain.ChannelEmail, "subj", "body").Return(nil, assert.AnError)

	c, rec := newCtx(http.MethodPut, "/api/v1/templates/"+id.String(), `{"name":"updated","channel":"email","subject":"subj","body":"body"}`, tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Update(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestTemplateHandler_Delete_Success(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Delete", mock.Anything, tenant.ID, id).Return(nil)

	c, rec := newCtx(http.MethodDelete, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestTemplateHandler_Delete_BadID(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()

	c, rec := newCtx(http.MethodDelete, "/api/v1/templates/bad", "", tenant)
	c.SetParamNames("id")
	c.SetParamValues("bad")
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestTemplateHandler_Delete_NotFound(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Delete", mock.Anything, tenant.ID, id).Return(postgres.ErrNotFound)

	c, rec := newCtx(http.MethodDelete, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestTemplateHandler_Delete_RepoError(t *testing.T) {
	repo := &mockTemplateRepo{}
	h := newTemplateHandler(repo)
	tenant := testTenant()
	id := uuid.New()

	repo.On("Delete", mock.Anything, tenant.ID, id).Return(assert.AnError)

	c, rec := newCtx(http.MethodDelete, "/api/v1/templates/"+id.String(), "", tenant)
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	err := h.Delete(c)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
