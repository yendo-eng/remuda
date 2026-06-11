package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yendo-eng/remuda/internal/configfile"
)

func TestSessionKillCmdValidateRejectsEmptyMergeFlags(t *testing.T) {
	t.Parallel()

	cmd := SessionKillCmd{
		SessionKillNamePickOption: SessionKillNamePickOption{Name: "acme-org/example-repo/test-session"},
		MergeFlag:                 []string{"--squash", " "},
	}

	err := cmd.Validate()
	require.ErrorContains(t, err, "--merge-flag[1] cannot be empty")
}

func TestSessionKillCmdConfiguredMergeFlagsForSession_UsesPerRepoDefaults(t *testing.T) {
	t.Parallel()

	globalFlags := []string{"--rebase"}
	perRepoFlags := []string{"--rebase", "--admin"}

	ctx := newTestContextWithEnv(t, EnvMap{})
	ctx.ConfigFile = &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Merge: &configfile.MergeV1{GHFlags: &globalFlags},
		},
		PerRepo: map[string]configfile.OverlayV1{
			"owner/repo": {
				Defaults: &configfile.DefaultsV1{
					Merge: &configfile.MergeV1{GHFlags: &perRepoFlags},
				},
			},
		},
	}

	cmd := SessionKillCmd{}
	flags := cmd.configuredMergeFlagsForSession(ctx, "owner/repo/workspace")

	require.Equal(t, []string{"--rebase", "--admin"}, flags)
	require.Equal(t, []string{"--rebase"}, *ctx.ConfigFile.Defaults.Merge.GHFlags)
}

func TestSessionKillCmdConfiguredMergeFlagsForSession_CLIFlagsWinOverPerRepoDefaults(t *testing.T) {
	t.Parallel()

	globalFlags := []string{"--rebase"}
	perRepoFlags := []string{"--rebase", "--admin"}

	ctx := newTestContextWithEnv(t, EnvMap{})
	ctx.ConfigFile = &configfile.V1{
		Version: 1,
		Defaults: &configfile.DefaultsV1{
			Merge: &configfile.MergeV1{GHFlags: &globalFlags},
		},
		PerRepo: map[string]configfile.OverlayV1{
			"owner/repo": {
				Defaults: &configfile.DefaultsV1{
					Merge: &configfile.MergeV1{GHFlags: &perRepoFlags},
				},
			},
		},
	}

	cmd := SessionKillCmd{MergeFlag: []string{"--squash", "--auto"}}
	flags := cmd.configuredMergeFlagsForSession(ctx, "owner/repo/workspace")

	require.Equal(t, []string{"--squash", "--auto"}, flags)
}
