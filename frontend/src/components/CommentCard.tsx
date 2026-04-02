import { useState, useEffect } from "react";
import { Check, X, FileCode } from "lucide-react";
import { Button } from "@/components/ui/button";
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

  // Use external editing state if provided, otherwise internal
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
        "rounded-lg border bg-surface p-4 transition-all duration-200",
        comment.approved
          ? "border-border"
          : "border-destructive/20 opacity-60",
        isFocused && "ring-2 ring-primary/50 border-primary/30"
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-3">
        <div className="flex items-center gap-3">
          <FileCode className="h-4 w-4 text-muted-foreground" />
          <span className="font-mono text-sm text-foreground">
            {comment.file}
          </span>
          <span className="font-mono text-xs text-muted-foreground">
            L{comment.line}
          </span>
          <SeverityBadge severity={comment.severity} />
        </div>
        <div className="flex items-center gap-1" role="group" aria-label="Comment approval">
          <Button
            variant={comment.approved ? "outline" : "ghost"}
            size="sm"
            aria-label="Approve comment"
            aria-pressed={comment.approved}
            onClick={() => !comment.approved && onToggleApproval(comment.id)}
            className={cn(
              "h-7 px-2",
              comment.approved &&
                "border-success/30 text-success hover:bg-success/10"
            )}
          >
            <Check className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant={!comment.approved ? "outline" : "ghost"}
            size="sm"
            aria-label="Reject comment"
            aria-pressed={!comment.approved}
            onClick={() => comment.approved && onToggleApproval(comment.id)}
            className={cn(
              "h-7 px-2",
              !comment.approved &&
                "border-destructive/30 text-destructive hover:bg-destructive/10"
            )}
          >
            <X className="h-3.5 w-3.5" />
          </Button>
        </div>
      </div>

      {/* Diff context */}
      <div className="mb-3">
        <DiffBlock
          lines={comment.diff_context}
          fileName={comment.file}
          startLine={Math.max(1, comment.line - 3)}
        />
      </div>

      {/* Comment body */}
      <div className="mt-3">
        {isEditing ? (
          <div className="space-y-2">
            <Textarea
              value={editValue}
              onChange={(e) => setEditValue(e.target.value)}
              className="font-mono text-sm bg-background border-border min-h-[80px] resize-y"
              autoFocus
            />
            <div className="flex gap-2">
              <Button size="sm" onClick={handleSaveEdit}>
                Save
              </Button>
              <Button size="sm" variant="ghost" onClick={handleCancelEdit}>
                Cancel
              </Button>
            </div>
          </div>
        ) : (
          <div
            role="button"
            tabIndex={0}
            onClick={handleStartEdit}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                handleStartEdit();
              }
            }}
            aria-label="Click to edit comment"
            className={cn(
              "cursor-pointer rounded-md bg-background px-3 py-2 text-sm text-foreground/80 hover:bg-muted transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              !comment.approved && "line-through text-muted-foreground"
            )}
          >
            {comment.body}
          </div>
        )}
      </div>
    </div>
  );
}
