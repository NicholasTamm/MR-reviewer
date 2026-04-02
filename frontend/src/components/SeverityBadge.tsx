import { cn } from "@/lib/utils";

interface SeverityBadgeProps {
  severity: "error" | "warning" | "info";
}

const config = {
  error: {
    label: "Error",
    className: "border-severity-error/30 text-severity-error bg-severity-error/10",
  },
  warning: {
    label: "Warning",
    className: "border-severity-warning/30 text-severity-warning bg-severity-warning/10",
  },
  info: {
    label: "Info",
    className: "border-severity-info/30 text-severity-info bg-severity-info/10",
  },
};

export function SeverityBadge({ severity }: SeverityBadgeProps) {
  const { label, className } = config[severity];

  return (
    <span
      className={cn(
        "inline-flex items-center rounded px-2 py-0.5 text-xs font-medium border",
        className
      )}
    >
      {label}
    </span>
  );
}
