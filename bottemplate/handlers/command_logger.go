package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/bot-template/bottemplate/config"
	"github.com/disgoorg/bot-template/bottemplate/services"
	"github.com/disgoorg/disgo/handler"
)

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

		// Execute the command with timeout tracking
		done := make(chan error, 1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					done <- fmt.Errorf("command panic: %v", r)
				}
			}()
			done <- h(e)
		}()

		// Wait for command completion or timeout
		select {
		case err := <-done:
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
			return err

		case <-time.After(10 * time.Second):
			slog.Error("Command timed out",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("status", "timeout"),
				slog.Duration("timeout", config.CommandExecutionTimeout),
			)
			return fmt.Errorf("command timed out after 10 seconds")
		}
	}
}

// WrapWithLoggingAndQuests wraps a command handler with logging and quest tracking
func WrapWithLoggingAndQuests(name string, h handler.CommandHandler, b interface{ GetQuestTracker() *services.QuestTracker }) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		// First apply the logging wrapper
		loggedHandler := WrapWithLogging(name, h)
		
		// Execute the command
		err := loggedHandler(e)
		
		// Track command for quests if successful
		if err == nil {
			if tracker := b.GetQuestTracker(); tracker != nil {
				// Run quest tracking in background to not slow down response
				slog.Debug("Tracking command for quest progress",
					slog.String("user_id", e.User().ID.String()),
					slog.String("command", name))
				go tracker.TrackCommand(context.Background(), e.User().ID.String(), name)
			} else {
				slog.Warn("Quest tracker is nil, cannot track command")
			}
		}
		
		return err
	}
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

		// Execute the component handler with timeout tracking
		done := make(chan error, 1)
		go func() {
			done <- h(e)
		}()

		// Wait for component completion or timeout
		select {
		case err := <-done:
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

		case <-time.After(10 * time.Second):
			slog.Error("Component interaction timed out",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("status", "timeout"),
				slog.Duration("timeout", config.CommandExecutionTimeout),
			)
			return fmt.Errorf("component interaction timed out after 10 seconds")
		}
	}
}
