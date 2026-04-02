import { useState } from "react";
import { Keyboard } from "lucide-react";

const shortcuts = [
  { keys: "j / k", description: "Navigate comments" },
  { keys: "a", description: "Approve comment" },
  { keys: "r", description: "Reject comment" },
  { keys: "e", description: "Edit comment" },
  { keys: "Esc", description: "Cancel edit" },
  { keys: "\u2318/Ctrl + \u23CE", description: "Post review" },
];

export function ShortcutHelp() {
  const [open, setOpen] = useState(false);

  return (
    <div className="fixed bottom-4 right-4 z-40">
      {open && (
        <div
          id="shortcut-help-panel"
          role="region"
          aria-label="Keyboard shortcuts"
          className="mb-2 rounded-lg border border-border bg-surface shadow-lg p-3 w-56 animate-in fade-in"
        >
          <div className="text-xs font-medium text-muted-foreground mb-2">
            Keyboard Shortcuts
          </div>
          <div className="space-y-1.5">
            {shortcuts.map((s) => (
              <div key={s.keys} className="flex items-center justify-between text-xs">
                <kbd className="rounded bg-muted px-1.5 py-0.5 font-mono text-foreground/80">
                  {s.keys}
                </kbd>
                <span className="text-muted-foreground">{s.description}</span>
              </div>
            ))}
          </div>
        </div>
      )}
      <button
        onClick={() => setOpen((prev) => !prev)}
        className="flex h-8 w-8 items-center justify-center rounded-full border border-border bg-surface text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
        aria-label="Keyboard shortcuts"
        aria-expanded={open}
        aria-controls="shortcut-help-panel"
        title="Keyboard shortcuts"
      >
        <Keyboard className="h-4 w-4" />
      </button>
    </div>
  );
}
