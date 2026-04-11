import { useState, useEffect } from "react";
import { Check, X, Pencil } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { SeverityBadge } from "@/components/SeverityBadge";
import { DiffBlock } from "@/components/DiffBlock";
import type { ReviewComment } from "@/types";
import { cn } from "@/lib/utils";

interface CommentCardProps {
  comment: ReviewComment;
  onToggleApproval: (id: string) => void;
  onEditBody: (id: string, newBody: string) => void;
  isFocused?: boolean;
  isEditing?: boolean;
  onStartEdit?: () => void;
  onCancelEdit?: () => void;
}

const severityStripe: Record<string, string> = {
  error: "bg-severity-error",
  warning: "bg-severity-warning",
  info: "bg-border",
};

export function CommentCard({
  comment,
  onToggleApproval,
  onEditBody,
  isFocused = false,
  isEditing: isEditingExternal,
  onStartEdit,
  onCancelEdit,
}: CommentCardProps) {
  const [isEditingInternal, setIsEditingInternal] = useState(false);
  const [editValue, setEditValue] = useState(comment.body);

  const isEditing = isEditingExternal !== undefined ? isEditingExternal : isEditingInternal;

  useEffect(() => {
    if (!isEditing) {
      setEditValue(comment.body);
    }
  }, [comment.body, isEditing]);

  const handleStartEdit = () => {
    if (onStartEdit) {
      onStartEdit();
    } else {
      setIsEditingInternal(true);
    }
  };

  const handleSaveEdit = () => {
    onEditBody(comment.id, editValue);
    if (onCancelEdit) {
      onCancelEdit();
    } else {
      setIsEditingInternal(false);
    }
  };

  const handleCancelEdit = () => {
    setEditValue(comment.body);
    if (onCancelEdit) {
      onCancelEdit();
    } else {
      setIsEditingInternal(false);
    }
  };

  return (
    <div
      className={cn(
        "flex rounded border bg-surface overflow-hidden transition-all duration-200",
        comment.approved ? "border-border" : "border-border/30 opacity-50",
        isFocused && "ring-1 ring-primary/40 border-primary/20"
      )}
    >
      {/* Left severity stripe */}
      <div className={cn("w-[3px] shrink-0 self-stretch", severityStripe[comment.severity])} />

      {/* Card content */}
      <div className="flex-1 min-w-0">
        {/* Header */}
        <div className="flex items-center justify-between px-4 pt-3 pb-2 gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <span className="font-mono text-xs text-foreground/80 truncate">
              {comment.file}
            </span>
            <span className="font-mono text-[10px] text-muted-foreground shrink-0">
              :{comment.line}
            </span>
          </div>
          <div className="shrink-0">
            <SeverityBadge severity={comment.severity} />
          </div>
        </div>

        {/* Diff context */}
        <div className="px-4 pb-3">
          <DiffBlock
            lines={comment.diff_context}
            fileName={comment.file}
            startLine={Math.max(1, comment.line - 3)}
          />
        </div>

        {/* Comment body */}
        <div className="px-4 pb-3">
          {isEditing ? (
            <div className="space-y-2">
              <Textarea
                value={editValue}
                onChange={(e) => setEditValue(e.target.value)}
                className="font-mono text-xs bg-background border-border min-h-[80px] resize-y leading-relaxed"
                autoFocus
                onKeyDown={(e) => {
                  if (e.key === "Enter" && (e.ctrlKey || e.metaKey)) {
                    e.preventDefault();
                    handleSaveEdit();
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    handleCancelEdit();
                  }
                }}
              />
              <div className="flex gap-2">
                <button
                  onClick={handleSaveEdit}
                  className="px-3 py-1 text-xs font-medium bg-primary text-primary-foreground rounded hover:bg-primary/90 transition-colors"
                >
                  Save
                </button>
                <button
                  onClick={handleCancelEdit}
                  className="px-3 py-1 text-xs font-medium text-muted-foreground hover:text-foreground transition-colors"
                >
                  Cancel
                </button>
              </div>
            </div>
          ) : (
            <div className="group relative">
              <p
                className={cn(
                  "text-sm text-foreground/80 leading-relaxed pr-6",
                  !comment.approved && "line-through text-muted-foreground"
                )}
              >
                {comment.body}
              </p>
              <button
                onClick={handleStartEdit}
                aria-label="Edit comment"
                className="absolute right-0 top-0 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground hover:text-foreground"
              >
                <Pencil className="h-3 w-3" />
              </button>
            </div>
          )}
        </div>

        {/* Footer: approve / reject */}
        <div className="flex border-t border-border" role="group" aria-label="Comment approval">
          <button
            onClick={() => !comment.approved && onToggleApproval(comment.id)}
            aria-label="Approve comment"
            aria-pressed={comment.approved}
            className={cn(
              "flex-1 flex items-center justify-center gap-1.5 py-2.5 text-xs font-medium transition-colors",
              comment.approved
                ? "bg-success/8 text-success"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
          >
            <Check className="h-3.5 w-3.5" />
            Approve
          </button>
          <div className="w-px bg-border" />
          <button
            onClick={() => comment.approved && onToggleApproval(comment.id)}
            aria-label="Reject comment"
            aria-pressed={!comment.approved}
            className={cn(
              "flex-1 flex items-center justify-center gap-1.5 py-2.5 text-xs font-medium transition-colors",
              !comment.approved
                ? "bg-destructive/8 text-destructive"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
          >
            <X className="h-3.5 w-3.5" />
            Reject
          </button>
        </div>
      </div>
    </div>
  );
}
