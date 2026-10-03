package main

import (
	"context"
	"testing"

	"github.com/renzaspiras/supabase/apps/supadata-platform/internal/project"
)

type recordingBucketProvisioner struct {
	buckets []string
}

func (p *recordingBucketProvisioner) EnsureBucket(_ context.Context, bucket string) error {
	p.buckets = append(p.buckets, bucket)
	return nil
}

func TestEnsureProjectBucketsUsesConfiguredIsolationScope(t *testing.T) {
	provisioner := &recordingBucketProvisioner{}
	projects := []project.Project{{
		ID: "default",
		Scope: project.ResourceScope{
			Storage: project.StorageScope{Bucket: "supadata-default"},
		},
	}}

	if err := ensureProjectBuckets(context.Background(), provisioner, projects); err != nil {
		t.Fatalf("ensureProjectBuckets() error = %v", err)
	}
	if len(provisioner.buckets) != 1 || provisioner.buckets[0] != "supadata-default" {
		t.Fatalf("EnsureBucket() buckets = %#v, want [supadata-default]", provisioner.buckets)
	}
}
