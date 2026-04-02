import { useEffect, useCallback } from "react";

interface KeyboardShortcutsOptions {
  /** Total number of comments */
  commentCount: number;
  /** Current focused comment index (-1 for none) */
  focusedIndex: number;
  /** Callback to set focused comment index */
  setFocusedIndex: (index: number) => void;
  /** Approve the comment at the given index */
  onApprove: (index: number) => void;
  /** Reject the comment at the given index */
  onReject: (index: number) => void;
  /** Start editing the comment at the given index */
  onEdit: (index: number) => void;
  /** Cancel current edit / close modal */
  onEscape: () => void;
  /** Post approved comments */
  onPost: () => void;
  /** Whether the page is in a state that accepts shortcuts */
  enabled: boolean;
}

function isInputElement(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  if (tagName === "input" || tagName === "textarea" || tagName === "select") {
    return true;
  }
  return target.isContentEditable;
}

export function useKeyboardShortcuts({
  commentCount,
  focusedIndex,
  setFocusedIndex,
  onApprove,
  onReject,
  onEdit,
  onEscape,
  onPost,
  enabled,
}: KeyboardShortcutsOptions) {
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (!enabled) return;

      // Escape always fires (closes modals, cancels edits)
      if (e.key === "Escape") {
        onEscape();
        return;
      }

      // Ctrl/Cmd+Enter fires from anywhere (even inputs)
      if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        onPost();
        return;
      }

      // All other shortcuts must not fire when typing in inputs
      if (isInputElement(e.target)) return;

      if (commentCount === 0) return;

      switch (e.key) {
        case "j": {
          e.preventDefault();
          const next =
            focusedIndex < commentCount - 1 ? focusedIndex + 1 : focusedIndex;
          setFocusedIndex(next);
          break;
        }
        case "k": {
          e.preventDefault();
          const prev = focusedIndex > 0 ? focusedIndex - 1 : 0;
          setFocusedIndex(prev);
          break;
        }
        case "a": {
          if (focusedIndex >= 0 && focusedIndex < commentCount) {
            e.preventDefault();
            onApprove(focusedIndex);
          }
          break;
        }
        case "r": {
          if (focusedIndex >= 0 && focusedIndex < commentCount) {
            e.preventDefault();
            onReject(focusedIndex);
          }
          break;
        }
        case "e": {
          if (focusedIndex >= 0 && focusedIndex < commentCount) {
            e.preventDefault();
            onEdit(focusedIndex);
          }
          break;
        }
      }
    },
    [
      enabled,
      commentCount,
      focusedIndex,
      setFocusedIndex,
      onApprove,
      onReject,
      onEdit,
      onEscape,
      onPost,
    ],
  );

  useEffect(() => {
    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleKeyDown]);
}
