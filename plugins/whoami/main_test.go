package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func fakeClient(ui authenticationv1.UserInfo, fail bool) *fake.Clientset {
	client := fake.NewSimpleClientset()
	client.PrependReactor("create", "selfsubjectreviews", func(k8stesting.Action) (bool, runtime.Object, error) {
		if fail {
			return true, nil, errors.New("boom")
		}
		return true, &authenticationv1.SelfSubjectReview{Status: authenticationv1.SelfSubjectReviewStatus{UserInfo: ui}}, nil
	})
	return client
}

var info = clusterInfo{Context: "dev", Cluster: "dev-cluster", Server: "https://example:6443"}

func TestRunText(t *testing.T) {
	client := fakeClient(authenticationv1.UserInfo{
		Username: "alice",
		UID:      "u1",
		Groups:   []string{"system:masters", "dev", "system:authenticated"},
		Extra:    map[string]authenticationv1.ExtraValue{"scopes": {"a"}},
	}, false)
	var out bytes.Buffer
	if err := run(context.Background(), client, &out, info, options{}); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"Context:  dev", "Cluster:  dev-cluster", "Server:   https://example:6443", "Username: alice", "UID:      u1", "  dev\n  system:authenticated\n  system:masters", "scopes"} {
		if !strings.Contains(out.String(), s) {
			t.Errorf("output missing %q\ngot:\n%s", s, out.String())
		}
	}
}

func TestRunJSON(t *testing.T) {
	client := fakeClient(authenticationv1.UserInfo{Username: "bob"}, false)
	var out bytes.Buffer
	if err := run(context.Background(), client, &out, info, options{output: "json"}); err != nil {
		t.Fatal(err)
	}
	var got identity
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out.String())
	}
	if got.Username != "bob" || got.Context != "dev" || got.Groups == nil {
		t.Errorf("unexpected identity: %+v", got)
	}
}

func TestRunError(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), fakeClient(authenticationv1.UserInfo{}, true), &out, info, options{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewCmdArgs(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "help", args: []string{"--help"}},
		{name: "unexpected positional arg", args: []string{"extra"}, wantErr: true},
		{name: "unknown flag", args: []string{"--nope"}, wantErr: true},
		{name: "bad output format", args: []string{"-o", "yaml"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			cmd := newCmd(&out)
			cmd.SetArgs(tt.args)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			if err := cmd.Execute(); (err != nil) != tt.wantErr {
				t.Fatalf("Execute() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
