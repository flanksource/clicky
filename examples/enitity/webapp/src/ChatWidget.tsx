import {
  ChatWindowManagerProvider,
  ChatFab,
  ChatWindowLayer,
  type ContextTypeConfig,
} from "@flanksource/clicky-ui/ai";

const TYPE_CONFIG: ContextTypeConfig = {
  stack: { icon: "ph:stack", className: "text-blue-600 bg-blue-50" },
  cluster: { icon: "ph:cloud", className: "text-emerald-600 bg-emerald-50" },
  team: { icon: "ph:users-three", className: "text-violet-600 bg-violet-50" },
};

/** Multi-window chat shell for the entity demo: a launch FAB plus up to six
 *  draggable/resizable windows, each hosting clicky-ui's <Chat> against the
 *  demo's /api/chat endpoint (where entity operations are exposed as tools) and
 *  switching between persisted threads via /api/chat/threads. */
export function ChatWidget() {
  return (
    <ChatWindowManagerProvider storageId="entity-demo">
      <ChatFab />
      <ChatWindowLayer
        threadsApi="/api/chat/threads"
        contextTypeConfig={TYPE_CONFIG}
        chat={{
          api: "/api/chat",
          modelsApi: "/api/chat/models",
          enableAttachments: true,
          toolApproval: "manual",
          suggestions: [
            "List all stacks",
            "Show clusters in the prod stack",
            { label: "Restart api", prompt: "Restart the api service" },
          ],
          placeholder: "Ask about stacks, clusters, or teams…",
          emptyState: (
            <div className="px-4 py-6 text-center text-sm text-muted-foreground">
              Ask the assistant to list, inspect, or act on entities.
            </div>
          ),
        }}
      />
    </ChatWindowManagerProvider>
  );
}
