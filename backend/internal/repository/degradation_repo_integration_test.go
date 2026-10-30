//go:build integration

package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// 降智检测的原生 SQL 路径（DISTINCT ON、octet_length、区间运算、部分唯一索引）
// 只有在真实 PostgreSQL 上跑过迁移 261/262 才能验证，单测里的桩覆盖不到。
func TestDegradationRepositoryAgainstMigratedPostgres(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := NewDegradationRepository(client, integrationDB)
	suffix := time.Now().UnixNano()

	group, err := client.Group.Create().
		SetName(fmt.Sprintf("degradation-it-%d", suffix)).
		SetPlatform(service.PlatformAnthropic).
		Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM degradation_runs WHERE group_id = $1", group.ID)
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM degradation_targets WHERE group_id = $1", group.ID)
		_, _ = integrationDB.ExecContext(bg, "DELETE FROM groups WHERE id = $1", group.ID)
	})

	regexProbe := &service.DegradationProbe{Name: "it-regex", Prompt: "1+1=?", ExpectRegex: `^2$`, Enabled: true, ProbeType: service.DegradationProbeTypeRegex}
	visualProbe := &service.DegradationProbe{Name: "it-visual", Prompt: "draw", Enabled: true, ProbeType: service.DegradationProbeTypeVisual, RunIntervalSeconds: 86400}
	require.NoError(t, repo.CreateProbe(ctx, regexProbe))
	require.NoError(t, repo.CreateProbe(ctx, visualProbe))
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM degradation_probes WHERE id IN ($1, $2)", regexProbe.ID, visualProbe.ID)
	})

	// 262 新增列在真实库里可读回。
	gotVisual, err := repo.GetProbe(ctx, visualProbe.ID)
	require.NoError(t, err)
	require.Equal(t, service.DegradationProbeTypeVisual, gotVisual.ProbeType)
	require.Equal(t, 86400, gotVisual.RunIntervalSeconds)

	target := &service.DegradationTarget{GroupID: group.ID, Model: "claude-it", ProbeIDs: []int64{}, IntervalSeconds: 300, Enabled: true}
	require.NoError(t, repo.CreateTarget(ctx, target))

	// 部分唯一索引：同分组同模型重复创建必须 409。
	dup := &service.DegradationTarget{GroupID: group.ID, Model: "claude-it", ProbeIDs: []int64{}, IntervalSeconds: 300, Enabled: true}
	require.ErrorIs(t, repo.CreateTarget(ctx, dup), service.ErrDegradationTargetDuplicate)

	// 从未运行的目标立即到期（last_run_at IS NULL）。
	due, err := repo.ListDueTargets(ctx, time.Now())
	require.NoError(t, err)
	require.True(t, containsDegradationTarget(due, target.ID), "never-run target must be due")

	// 刚运行过 → 区间运算 last_run_at + interval_seconds * '1 second' 判定为未到期。
	require.NoError(t, repo.UpdateTargetLastRun(ctx, target.ID, time.Now()))
	due, err = repo.ListDueTargets(ctx, time.Now())
	require.NoError(t, err)
	require.False(t, containsDegradationTarget(due, target.ID), "just-run target must not be due")
	due, err = repo.ListDueTargets(ctx, time.Now().Add(301*time.Second))
	require.NoError(t, err)
	require.True(t, containsDegradationTarget(due, target.ID), "target must be due after its interval")

	html := "<!doctype html><p>" + strings.Repeat("鹈", 10) + "</p>"
	older := &service.DegradationRun{TargetID: target.ID, GroupID: group.ID, Model: "claude-it", ProbeID: visualProbe.ID, ProbeName: "it-visual",
		ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunRendered, ArtifactKind: service.DegradationArtifactHTML, ArtifactContent: "<!doctype html><p>old</p>"}
	newer := &service.DegradationRun{TargetID: target.ID, GroupID: group.ID, Model: "claude-it", ProbeID: visualProbe.ID, ProbeName: "it-visual",
		ProbeType: service.DegradationProbeTypeVisual, Status: service.DegradationRunRendered, ArtifactKind: service.DegradationArtifactHTML, ArtifactContent: html}
	regexRun := &service.DegradationRun{TargetID: target.ID, GroupID: group.ID, Model: "claude-it", ProbeID: regexProbe.ID, ProbeName: "it-regex",
		ProbeType: service.DegradationProbeTypeRegex, Status: service.DegradationRunPassed, AnswerText: "2"}
	require.NoError(t, repo.CreateRun(ctx, older))
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, repo.CreateRun(ctx, newer))
	require.NoError(t, repo.CreateRun(ctx, regexRun))

	// 列表不得带出作品原文，但 artifact_bytes 必须是 UTF-8 字节数（octet_length），不是字符数。
	byTarget, err := repo.ListRunsByTarget(ctx, target.ID, 50)
	require.NoError(t, err)
	require.Len(t, byTarget, 3)
	for _, run := range byTarget {
		require.Empty(t, run.ArtifactContent, "list must never load artifact_content")
		if run.ID == newer.ID {
			require.Equal(t, int64(len(html)), run.ArtifactBytes)
		}
	}

	// 渲染路径才读原文。
	artifact, err := repo.GetRunArtifact(ctx, newer.ID)
	require.NoError(t, err)
	require.Equal(t, html, artifact.ArtifactContent)
	plain, err := repo.GetRun(ctx, newer.ID)
	require.NoError(t, err)
	require.Empty(t, plain.ArtifactContent)

	// DISTINCT ON：每个 (分组, 模型, 题目) 只取最新一条 rendered。
	latest, err := repo.ListLatestVisualRuns(ctx, []int64{group.ID})
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.Equal(t, newer.ID, latest[0].ID)
	require.Empty(t, latest[0].ArtifactContent)

	history, err := repo.ListVisualRuns(ctx, group.ID, "claude-it", visualProbe.ID, 20)
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, newer.ID, history[0].ID, "history must be newest first")

	// 调度频率依据：每个题目最近一次运行时间。
	lastRuns, err := repo.LatestProbeRuns(ctx, target.ID, []int64{visualProbe.ID, regexProbe.ID})
	require.NoError(t, err)
	require.Len(t, lastRuns, 2)
	require.False(t, lastRuns[visualProbe.ID].Before(lastRuns[regexProbe.ID].Add(-time.Minute)))

	// 用户端流水：只出启用目标的模型，且不含回答/错误/账号等敏感列。
	userRuns, err := repo.ListRuns(ctx, []int64{group.ID}, "", time.Now().Add(-time.Hour), 50, false)
	require.NoError(t, err)
	require.Len(t, userRuns, 3)
	for _, run := range userRuns {
		require.Empty(t, run.AnswerText)
		require.Empty(t, run.ErrorMessage)
		require.Zero(t, run.AccountID)
		require.Empty(t, run.ArtifactContent)
	}

	// 删除分组 → 目标随之软删，部分唯一索引允许重建同名目标。
	require.NoError(t, repo.SoftDeleteTargetsByGroupID(ctx, group.ID))
	_, err = repo.GetTarget(ctx, target.ID)
	require.ErrorIs(t, err, service.ErrDegradationTargetNotFound)
	recreated := &service.DegradationTarget{GroupID: group.ID, Model: "claude-it", ProbeIDs: []int64{}, IntervalSeconds: 300, Enabled: true}
	require.NoError(t, repo.CreateTarget(ctx, recreated))

	// 30 天保留：清理只删旧数据。
	_, err = integrationDB.ExecContext(ctx, "UPDATE degradation_runs SET created_at = NOW() - INTERVAL '31 days' WHERE id = $1", older.ID)
	require.NoError(t, err)
	deleted, err := repo.DeleteRunsBefore(ctx, time.Now().Add(-30*24*time.Hour))
	require.NoError(t, err)
	require.GreaterOrEqual(t, deleted, int64(1))
	_, err = repo.GetRun(ctx, older.ID)
	require.Error(t, err)
	_, err = repo.GetRun(ctx, newer.ID)
	require.NoError(t, err)
}

func containsDegradationTarget(targets []*service.DegradationTarget, id int64) bool {
	for _, target := range targets {
		if target.ID == id {
			return true
		}
	}
	return false
}
