import { useState } from "react";
import { Icon } from "@flanksource/clicky-ui";
import { Chat } from "@flanksource/clicky-ui/chat";

/** Always-on chat launcher pinned to the bottom-right corner. The button
 *  toggles a floating panel that hosts clicky-ui's <Chat> against the demo's
 *  /api/chat endpoint, where the demo's entity operations are exposed as tools. */
export function ChatWidget() {
  const [open, setOpen] = useState(false);

  return (
    <>
      {open ? (
        <div className="fixed bottom-24 right-6 z-50 flex h-[600px] max-h-[calc(100vh-8rem)] w-96 max-w-[calc(100vw-3rem)] flex-col overflow-hidden rounded-2xl border border-border/70 bg-background shadow-2xl">
          <div className="flex items-center justify-between border-b border-border/70 px-4 py-2.5">
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Icon name="ph:sparkle" className="text-muted-foreground" />
              Assistant
            </div>
            <button
              type="button"
              aria-label="Close chat"
              onClick={() => setOpen(false)}
              className="rounded-full p-1 text-muted-foreground hover:bg-accent hover:text-accent-foreground"
            >
              <Icon name="ph:x" />
            </button>
          </div>
          <div className="min-h-0 flex-1">
            <Chat
              api="/api/chat"
              modelsApi="/api/chat/models"
              enableAttachments
              toolApproval="manual"
              suggestions={[
                "List all stacks",
                "Show clusters in the prod stack",
                { label: "Restart api", prompt: "Restart the api service" },
              ]}
              placeholder="Ask about stacks, clusters, or teams…"
              emptyState={
                <div className="px-4 py-6 text-center text-sm text-muted-foreground">
                  Ask the assistant to list, inspect, or act on entities.
                </div>
              }
            />
          </div>
        </div>
      ) : null}

      <button
        type="button"
        aria-label={open ? "Close chat" : "Open chat"}
        onClick={() => setOpen((v) => !v)}
        className="fixed bottom-6 right-6 z-50 flex h-14 w-14 items-center justify-center rounded-full bg-foreground text-background shadow-lg transition-transform hover:scale-105"
      >
        <Icon name={open ? "ph:x" : "ph:chat-circle"} className="text-2xl" />
      </button>
    </>
  );
}
