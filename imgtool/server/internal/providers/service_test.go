package providers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"imgtool/server/internal/auth"
)

const testServiceSecret = "0123456789abcdef0123456789abcdef"

func TestServiceListAndResolveUsesPortalCredentialsAndCoreModels(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/v1/integrations/imgtool/credentials" {
			t.Fatalf("portal request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != `{"user_id":7}` {
			t.Fatalf("portal body = %s", body)
		}
		if r.Header.Get("X-Imgtool-Timestamp") == "" || r.Header.Get("X-Imgtool-Signature") == "" {
			t.Fatal("portal request is missing service authentication headers")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"providers":[
            {"id":"openai","label":"OpenAI","group_id":15,"group_name":"gpt-image-2【5分钱一张】","key":"sk-openai"},
            {"id":"grok","label":"Grok","group_id":33,"group_name":"Grok Heavy","key":"sk-grok"}
        ]}}`))
	}))
	defer portal.Close()

	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Fatalf("core request = %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer sk-") {
			t.Fatalf("core authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[
            {"id":"gpt-4o"},
            {"id":"gpt-image-1"},
            {"id":"gpt-image-2"},
            {"id":"grok-imagine-video"},
            {"id":"grok-imagine-image-2.0"},
            {"id":"grok-imagine-image-quality"}
        ]}`))
	}))
	defer core.Close()

	service := NewService(Options{
		PortalBaseURL: portal.URL,
		SSOSecret:     testServiceSecret,
		CoreBaseURL:   core.URL,
		Now:           func() time.Time { return time.Unix(1700000000, 0) },
	})
	user := auth.User{ExternalProvider: "sub2api", ExternalSubject: "7"}

	views, err := service.List(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || !views[0].Available || !views[1].Available {
		t.Fatalf("views = %#v", views)
	}
	if views[0].DefaultModelID != "gpt-image-2" || views[1].DefaultModelID != "grok-imagine-image-2.0" {
		t.Fatalf("defaults = %q, %q", views[0].DefaultModelID, views[1].DefaultModelID)
	}
	if views[0].Name != "gpt-image-2【5分钱一张】" || views[1].Name != "Grok Heavy" {
		t.Fatalf("group names = %q, %q", views[0].Name, views[1].Name)
	}

	snapshot, err := service.ResolveImage("usr_1", "7", "openai", "gpt-image-2", "text-to-image")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.APIKey != "sk-openai" || snapshot.BaseURL != core.URL || snapshot.ModelID != "gpt-image-2" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestServiceRejectsModelOutsideFilteredProviderList(t *testing.T) {
	portal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"providers":[{"id":"openai","label":"OpenAI","group_id":15,"key":"sk-openai"}]}}`))
	}))
	defer portal.Close()
	core := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-image-2"}]}`))
	}))
	defer core.Close()

	service := NewService(Options{PortalBaseURL: portal.URL, SSOSecret: testServiceSecret, CoreBaseURL: core.URL})
	_, err := service.ResolveImage("usr_1", "7", "openai", "gpt-4o", "text-to-image")
	if err != ErrUnavailable {
		t.Fatalf("error = %v, want ErrUnavailable", err)
	}
}
