package gitlaboauth

import (
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/sergi/go-diff/diffmatchpatch"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth/providers"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/envvar"
	"github.com/sourcegraph/sourcegraph/enterprise/cmd/frontend/internal/auth/oauth"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestParseConfig(t *testing.T) {
	spew.Config.DisablePointerAddresses = true
	spew.Config.SortKeys = true
	spew.Config.SpewKeys = true

	type args struct {
		cfg *conf.Unified
	}
	tests := []struct {
		name          string
		args          args
		dotcom        bool
		wantProviders map[schema.GitLabAuthProvider]providers.Provider
		wantProblems  []string
	}{
		{
			name:          "No configs",
			args:          args{cfg: &conf.Unified{}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{},
		},
		{
			name: "1 GitLab.com config",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{
				{
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					DisplayName:  "GitLab",
					Type:         extsvc.TypeGitLab,
					Url:          "https://gitlab.com",
				}: provider("https://gitlab.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://gitlab.com/oauth/authorize",
						TokenURL: "https://gitlab.com/oauth/token",
					},
					Scopes: []string{"read_user", "api"},
				}),
			},
		},
		{
			name:   "1 GitLab.com config, Sourcegraph.com",
			dotcom: true,
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}},
			}}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{
				{
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					DisplayName:  "GitLab",
					Type:         extsvc.TypeGitLab,
					Url:          "https://gitlab.com",
				}: provider("https://gitlab.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://gitlab.com/oauth/authorize",
						TokenURL: "https://gitlab.com/oauth/token",
					},
					Scopes: []string{"read_user", "read_api"},
				}),
			},
		},
		{
			name: "2 GitLab configs",
			args: args{cfg: &conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				ExternalURL: "https://sourcegraph.example.com",
				AuthProviders: []schema.AuthProviders{{
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id",
						ClientSecret: "my-client-secret",
						DisplayName:  "GitLab",
						Type:         extsvc.TypeGitLab,
						Url:          "https://gitlab.com",
					},
				}, {
					Gitlab: &schema.GitLabAuthProvider{
						ClientID:     "my-client-id-2",
						ClientSecret: "my-client-secret-2",
						DisplayName:  "GitLab Enterprise",
						Type:         extsvc.TypeGitLab,
						Url:          "https://mycompany.com",
					},
				}},
			}}},
			wantProviders: map[schema.GitLabAuthProvider]providers.Provider{
				{
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					DisplayName:  "GitLab",
					Type:         extsvc.TypeGitLab,
					Url:          "https://gitlab.com",
				}: provider("https://gitlab.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id",
					ClientSecret: "my-client-secret",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://gitlab.com/oauth/authorize",
						TokenURL: "https://gitlab.com/oauth/token",
					},
					Scopes: []string{"read_user", "api"},
				}),
				{
					ClientID:     "my-client-id-2",
					ClientSecret: "my-client-secret-2",
					DisplayName:  "GitLab Enterprise",
					Type:         extsvc.TypeGitLab,
					Url:          "https://mycompany.com",
				}: provider("https://mycompany.com/", oauth2.Config{
					RedirectURL:  "https://sourcegraph.example.com/.auth/gitlab/callback",
					ClientID:     "my-client-id-2",
					ClientSecret: "my-client-secret-2",
					Endpoint: oauth2.Endpoint{
						AuthURL:  "https://mycompany.com/oauth/authorize",
						TokenURL: "https://mycompany.com/oauth/token",
					},
					Scopes: []string{"read_user", "api"},
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := envvar.SourcegraphDotComMode()
			envvar.MockSourcegraphDotComMode(tt.dotcom)
			t.Cleanup(func() {
				envvar.MockSourcegraphDotComMode(old)
			})

			gotProviders, gotProblems := parseConfig(tt.args.cfg)
			gotConfigs := make(map[schema.GitLabAuthProvider]oauth2.Config)
			for k, p := range gotProviders {
				if p, ok := p.(*oauth.Provider); ok {
					p.Login, p.Callback = nil, nil
					gotConfigs[k] = p.OAuth2Config()
					p.OAuth2Config = nil
					p.ProviderOp.Login, p.ProviderOp.Callback = nil, nil
				}
			}
			wantConfigs := make(map[schema.GitLabAuthProvider]oauth2.Config)
			for k, p := range tt.wantProviders {
				k := k
				if q, ok := p.(*oauth.Provider); ok {
					q.SourceConfig = schema.AuthProviders{Gitlab: &k}
					wantConfigs[k] = q.OAuth2Config()
					q.OAuth2Config = nil
				}
			}
			if !reflect.DeepEqual(gotProviders, tt.wantProviders) {
				dmp := diffmatchpatch.New()
				t.Errorf("parseConfig() gotProviders != tt.wantProviders, diff:\n%s",
					dmp.DiffPrettyText(dmp.DiffMain(spew.Sdump(tt.wantProviders), spew.Sdump(gotProviders), false)),
				)
			}
			if !reflect.DeepEqual(gotProblems.Messages(), tt.wantProblems) {
				t.Errorf("parseConfig() gotProblems = %v, want %v", gotProblems, tt.wantProblems)
			}

			if !reflect.DeepEqual(gotConfigs, wantConfigs) {
				dmp := diffmatchpatch.New()
				t.Errorf("parseConfig() gotConfigs != wantConfigs, diff:\n%s",
					dmp.DiffPrettyText(dmp.DiffMain(spew.Sdump(gotConfigs), spew.Sdump(wantConfigs), false)),
				)
			}
		})
	}
}

func provider(serviceID string, oauth2Config oauth2.Config) *oauth.Provider {
	op := oauth.ProviderOp{
		AuthPrefix:   authPrefix,
		OAuth2Config: func(extraScopes ...string) oauth2.Config { return oauth2Config },
		StateConfig:  getStateConfig(),
		ServiceID:    serviceID,
		ServiceType:  extsvc.TypeGitLab,
	}
	return &oauth.Provider{ProviderOp: op}
}
