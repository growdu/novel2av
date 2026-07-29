package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

// IngestTaskResult routes a Celery task completion callback from the
// ai-engine worker to the appropriate ingest call on the matching
// sub-service. Unknown task names return an error so we don't silently
// swallow notifications.
func (s *Services) IngestTaskResult(
	ctx context.Context,
	taskName string,
	_ string, // taskID; reserved for future per-job tracking
	projectID string,
	payload json.RawMessage,
	result json.RawMessage,
	errMsg string,
) (any, error) {
	start := time.Now()
	defer func() {
		if PipelineIngestDuration != nil {
			PipelineIngestDuration.WithLabelValues(taskName).Observe(time.Since(start).Seconds())
		}
	}()
	bumpOutcome := func(outcome string) {
		if PipelineJobsCompleted != nil {
			PipelineJobsCompleted.WithLabelValues(taskName, outcome).Inc()
		}
	}
	bumpErrOutcome := func(err error) {
		if err == nil {
			bumpOutcome("success")
			return
		}
		bumpOutcome("failure")
	}
	if errMsg != "" {
		// Failure path: backend currently does not have a "mark job failed"
		// ingest surface (Celery owns retry). We log here so the failure is
		// at least observable, then return 200 so the worker stops retrying
		// the notify call.
		// TODO(M8): persist failure to Job.meta + publish job.failed WS event.
		slog.Warn("task failed callback from ai-engine",
			"task", taskName, "project_id", projectID, "err", errMsg)
		if PipelineProvidersFailures != nil {
			PipelineProvidersFailures.WithLabelValues("unknown").Inc()
		}
		bumpOutcome("ignored_task_failed")
		return map[string]any{"ignored": "task_failed"}, nil
	}

	// dispatch returns (payload, error). The outcome counter is bumped by
	// the surrounding caller based on the resulting err so all branches
	// are instrumented without scattering helper calls.
	dispatch := func() (any, error) {
		switch taskName {
		case "ai:split_chapters":
			n, err := s.Chapter.IngestSplitResult(ctx, projectID)
			return map[string]any{"ingested": n}, err

		case "ai:extract_characters":
			n, err := s.Character.IngestExtractResult(ctx, projectID)
			return map[string]any{"ingested": n}, err

		case "ai:character_image":
			var p struct {
				CharacterID string `json:"character_id"`
			}
			_ = json.Unmarshal(payload, &p)
			var r struct {
				RefImageKey string `json:"ref_image_key"`
			}
			_ = json.Unmarshal(result, &r)
			if p.CharacterID == "" {
				return nil, fmt.Errorf("character_image payload missing character_id")
			}
			if err := s.Character.IngestCharacterImage(ctx, p.CharacterID, r.RefImageKey); err != nil {
				return nil, err
			}
			return map[string]any{"character": p.CharacterID}, nil

		case "ai:scene_breakdown":
			var cid string
			_ = json.Unmarshal(payload, &struct {
				ChapterID string `json:"chapter_id"`
			}{ChapterID: cid})
			if cid == "" {
				return nil, fmt.Errorf("scene_breakdown payload missing chapter_id")
			}
			n, err := s.Shot.IngestBreakdown(ctx, projectID, cid)
			return map[string]any{"ingested": n}, err

		case "ai:generate_shot":
			var sid string
			_ = json.Unmarshal(payload, &struct {
				ShotID string `json:"shot_id"`
			}{ShotID: sid})
			if sid == "" {
				return nil, fmt.Errorf("generate_shot payload missing shot_id")
			}
			var r struct {
				ImageKey    string `json:"image_key"`
				TTSKey      string `json:"tts_key"`
				BGMKey      string `json:"bgm_key"`
				SrtKey      string `json:"srt_key"`
				SubtitleKey string `json:"subtitle_key"`
			}
			_ = json.Unmarshal(result, &r)
			if r.SubtitleKey == "" {
				r.SubtitleKey = r.SrtKey
			}
			patch := ShotAssetPatch{
				ImageKey:    strPtr(r.ImageKey),
				TTSKey:      strPtr(r.TTSKey),
				BGMKey:      strPtr(r.BGMKey),
				SubtitleKey: strPtr(r.SubtitleKey),
			}
			if _, err := s.Shot.IngestShotAssets(ctx, sid, patch); err != nil {
				return nil, err
			}
			return map[string]any{"shot": sid}, nil

		case "ai:compose_chapter":
			var cid string
			_ = json.Unmarshal(payload, &struct {
				ChapterID string `json:"chapter_id"`
			}{ChapterID: cid})
			if cid == "" {
				return nil, fmt.Errorf("compose_chapter payload missing chapter_id")
			}
			var r struct {
				VideoKey    string  `json:"video_key"`
				DurationSec float64 `json:"duration_sec"`
			}
			_ = json.Unmarshal(result, &r)
			if err := s.Composition.IngestComposeResult(ctx, cid, r.VideoKey, r.DurationSec, "READY", ""); err != nil {
				return nil, err
			}
			return map[string]any{"chapter": cid}, nil

		case "ai:compose_full":
			var r struct {
				VideoKey    string  `json:"video_key"`
				DurationSec float64 `json:"duration_sec"`
			}
			_ = json.Unmarshal(result, &r)
			if err := s.ProjectVideo.IngestComposeResult(ctx, projectID, r.VideoKey, r.DurationSec, "READY", ""); err != nil {
				return nil, err
			}
			return map[string]any{"project": projectID}, nil

		default:
			return nil, fmt.Errorf("unknown task: %s", taskName)
		}
	}
	out, derr := dispatch()
	bumpErrOutcome(derr)
	return out, derr
}

func strPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
