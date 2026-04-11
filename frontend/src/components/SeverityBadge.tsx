import { cn } from "@/lib/utils";

interface SeverityBadgeProps {
  severity: "error" | "warning" | "info";
}

const config = {
  error: {
    label: "error",
    dotClass: "bg-severity-error",
    textClass: "text-severity-error",
  },
  warning: {
    label: "warning",
    dotClass: "bg-severity-warning",
    textClass: "text-severity-warning",
  },
  info: {
    label: "info",
    dotClass: "bg-severity-info",
    textClass: "text-severity-info",
  },
};

export function SeverityBadge({ severity }: SeverityBadgeProps) {
  const { label, dotClass, textClass } = config[severity];

  return (
    <span className={cn("inline-flex items-center gap-1.5 font-mono text-[10px] uppercase tracking-widest", textClass)}>
      <span className={cn("h-1.5 w-1.5 rounded-full shrink-0", dotClass)} />
      {label}
    </span>
  );
}
