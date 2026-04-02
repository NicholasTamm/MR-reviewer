import { cn } from "@/lib/utils";

interface DiffBlockProps {
  lines: string[];
  fileName: string;
  startLine?: number;
}

export function DiffBlock({ lines, fileName, startLine = 1 }: DiffBlockProps) {
  return (
    <div className="rounded-md border border-border overflow-hidden">
      <div className="bg-muted px-3 py-1.5 text-xs font-mono text-muted-foreground border-b border-border">
        {fileName}
      </div>
      <div className="bg-diff-bg overflow-x-auto">
        <pre className="text-xs leading-5">
          {lines.map((line, index) => {
            const isAdded = line.startsWith("+");
            const isRemoved = line.startsWith("-");
            const lineNum = startLine + index;

            return (
              <div
                key={index}
                className={cn(
                  "flex font-mono",
                  isAdded && "bg-success/8",
                  isRemoved && "bg-destructive/8"
                )}
              >
                <span className="w-10 flex-shrink-0 text-right pr-3 select-none text-muted-foreground/40 border-r border-border/50 py-px">
                  {lineNum}
                </span>
                <span
                  className={cn(
                    "pl-3 pr-4 py-px flex-1 whitespace-pre",
                    isAdded && "text-success/90",
                    isRemoved && "text-destructive/80",
                    !isAdded && !isRemoved && "text-foreground/70"
                  )}
                >
                  {line}
                </span>
              </div>
            );
          })}
        </pre>
      </div>
    </div>
  );
}
