import { cn } from "@/lib/utils";

interface DiffBlockProps {
  lines: string[];
  fileName: string;
  startLine?: number;
}

export function DiffBlock({ lines, fileName, startLine = 1 }: DiffBlockProps) {
  return (
    <div className="rounded border border-border overflow-hidden">
      <div className="bg-muted/60 px-3 py-1.5 text-[10px] font-mono text-muted-foreground border-b border-border tracking-wide">
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
                  isAdded && "bg-success/7",
                  isRemoved && "bg-destructive/7"
                )}
              >
                <span className="w-10 flex-shrink-0 text-right pr-3 select-none text-muted-foreground/30 border-r border-border/40 py-px">
                  {lineNum}
                </span>
                <span
                  className={cn(
                    "pl-3 pr-4 py-px flex-1 whitespace-pre",
                    isAdded && "text-success/80",
                    isRemoved && "text-destructive/70",
                    !isAdded && !isRemoved && "text-foreground/60"
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
