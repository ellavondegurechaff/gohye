package handlers

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/handler"
)

// WrapWithLogging wraps a command handler with logging functionality
func WrapWithLogging(name string, h handler.CommandHandler) handler.CommandHandler {
	return func(e *handler.CommandEvent) error {
		start := time.Now()

		// Log command start
		slog.Info("Command started",
			slog.String("type", "cmd"),
			slog.String("name", name),
			slog.String("user_id", e.User().ID.String()),
			slog.String("user_name", e.User().Username),
			slog.String("guild_id", e.GuildID().String()),
			slog.String("channel_id", e.ChannelID().String()),
		)

		// Execute the command with timeout tracking
		done := make(chan error, 1)
		go func() {
			done <- h(e)
		}()

		// Wait for command completion or timeout
		select {
		case err := <-done:
			duration := time.Since(start)

			// Log command completion
			attrs := []any{
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
			}

			if err != nil {
				slog.Error("Command failed", append(attrs,
					slog.Any("error", err),
					slog.String("status", "failed"),
				)...)
			} else {
				if duration > 2*time.Second {
					slog.Warn("Command executed slowly", append(attrs,
						slog.String("status", "slow"),
					)...)
				} else {
					slog.Info("Command completed", append(attrs,
						slog.String("status", "success"),
					)...)
				}
			}
			return err

		case <-time.After(10 * time.Second):
			slog.Error("Command timed out",
				slog.String("type", "cmd"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("status", "timeout"),
				slog.Duration("timeout", 10*time.Second),
			)
			return fmt.Errorf("command timed out after 10 seconds")
		}
	}
}

// WrapComponentWithLogging wraps a component handler with logging functionality
func WrapComponentWithLogging(name string, h handler.ComponentHandler) handler.ComponentHandler {
	return func(e *handler.ComponentEvent) error {
		start := time.Now()

		// Log component interaction start
		slog.Info("Component interaction started",
			slog.String("type", "component"),
			slog.String("name", name),
			slog.String("user_id", e.User().ID.String()),
			slog.String("user_name", e.User().Username),
			slog.String("guild_id", e.GuildID().String()),
			slog.String("channel_id", e.ChannelID().String()),
		)

		// Execute the component handler with timeout tracking
		done := make(chan error, 1)
		go func() {
			done <- h(e)
		}()

		// Wait for component completion or timeout
		select {
		case err := <-done:
			duration := time.Since(start)

			// Log component completion
			attrs := []any{
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.Duration("took", duration),
			}

			if err != nil {
				slog.Error("Component interaction failed", append(attrs,
					slog.Any("error", err),
					slog.String("status", "failed"),
				)...)
			} else {
				if duration > 2*time.Second {
					slog.Warn("Component interaction executed slowly", append(attrs,
						slog.String("status", "slow"),
					)...)
				} else {
					slog.Info("Component interaction completed", append(attrs,
						slog.String("status", "success"),
					)...)
				}
			}
			return err

		case <-time.After(10 * time.Second):
			slog.Error("Component interaction timed out",
				slog.String("type", "component"),
				slog.String("name", name),
				slog.String("user_id", e.User().ID.String()),
				slog.String("user_name", e.User().Username),
				slog.String("status", "timeout"),
				slog.Duration("timeout", 10*time.Second),
			)
			return fmt.Errorf("component interaction timed out after 10 seconds")
		}
	}
}
