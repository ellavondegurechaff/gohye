package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/bot-template/bottemplate/utils"
	"github.com/disgoorg/disgo/handler"
)

var questTrackerStore struct {
	sync.RWMutex
	tracker *services.QuestTracker
}

// SetQuestTracker wires command logging into quest progress tracking.
func SetQuestTracker(tracker *services.QuestTracker) {
	questTrackerStore.Lock()
	defer questTrackerStore.Unlock()
	questTrackerStore.tracker = tracker
}

func getQuestTracker() *services.QuestTracker {
	questTrackerStore.RLock()
	defer questTrackerStore.RUnlock()
	return questTrackerStore.tracker
}

// WrapWithLogging wraps a command handler with logging functionality
func WrapWithLogging(name string, h handler.CommandHandler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()

		// Log command start only for debug level
		if slog.Default().Enabled(nil, slog.LevelDebug) {
			slog.Debug("Command started",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("guild_id", e.GuildID().String()),
				slog.String("channel_id", e.ChannelID().String()),
			)
		}

		err := runCommandHandler(h, e)
		duration := time.Since(start)

		// Log command completion with optimized level checking
		if err != nil {
			// Always log errors
			slog.Error("Command failed",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.Any("error", err),
				slog.String("status", "failed"),
			)
		} else if duration > 2*time.Second {
			// Always log slow commands
			slog.Warn("Command executed slowly",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.String("status", "slow"),
			)
		} else if slog.Default().Enabled(nil, slog.LevelDebug) {
			// Only log successful completions at debug level
			slog.Debug("Command completed",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.String("status", "success"),
			)
		}
		if err == nil {
			if tracker := getQuestTracker(); tracker != nil {
				go tracker.TrackCommand(context.Background(), e.User().ID.String(), name)
			}
		}
		return err
	}
}

func runCommandHandler(h handler.CommandHandler, e *handler.CommandEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("command panic: %v", r)
		}
	}()
	return h(e)
}

// WrapWithLoggingAndQuests wraps a command handler with logging and quest tracking
func WrapWithLoggingAndQuests(name string, h handler.CommandHandler, b interface{ GetQuestTracker() *services.QuestTracker }) handler.CommandHandler {
	_ = b
	return WrapWithLogging(name, h)
}

// WrapComponentWithLogging wraps a component handler with logging functionality
func WrapComponentWithLogging(name string, h handler.ComponentHandler) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		start := time.Now()

		// Log component interaction start only for debug level
		if slog.Default().Enabled(nil, slog.LevelDebug) {
			slog.Debug("Component interaction started",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("guild_id", e.GuildID().String()),
				slog.String("channel_id", e.ChannelID().String()),
			)
		}

		err := runComponentHandler(h, e, name)
		duration := time.Since(start)

		// Log component completion with optimized level checking
		if err != nil {
			// Always log errors
			slog.Error("Component interaction failed",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.Any("error", err),
				slog.String("status", "failed"),
			)
		} else if duration > 2*time.Second {
			// Always log slow interactions
			slog.Warn("Component interaction executed slowly",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.String("status", "slow"),
			)
		} else if slog.Default().Enabled(nil, slog.LevelDebug) {
			// Only log successful completions at debug level
			slog.Debug("Component interaction completed",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
				slog.String("status", "success"),
			)
		}
		return err
	}
}

func runComponentHandler(h handler.ComponentHandler, e *handler.ComponentEvent, name string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("Component panic recovered",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.Any("panic", r),
			)
			_ = utils.EH.CreateEphemeralError(e, "Something went wrong while handling your action. Please try again.")
			err = fmt.Errorf("component panic: %v", r)
		}
	}()
	return h(e)
}
