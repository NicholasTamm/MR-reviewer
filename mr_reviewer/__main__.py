import argparse
import logging
import sys

from mr_reviewer.core import review_mr


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="mr_reviewer",
        description="AI-powered GitLab merge request reviewer using Claude",
    )
    parser.add_argument(
        "url",
        help="GitLab MR URL (e.g., https://gitlab.com/group/project/-/merge_requests/1)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print review to stdout instead of posting to GitLab",
    )
    parser.add_argument(
        "--focus",
        default="bugs,style,best-practices",
        help="Comma-separated review focus areas (default: bugs,style,best-practices)",
    )
    parser.add_argument(
        "--model",
        default="claude-sonnet-4-20250514",
        help="Claude model to use (default: claude-sonnet-4-20250514)",
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",
        help="Enable verbose logging",
    )

    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(levelname)s: %(message)s",
    )

    focus = [f.strip() for f in args.focus.split(",")]

    try:
        review_mr(
            url=args.url,
            focus=focus,
            dry_run=args.dry_run,
            model=args.model,
        )
    except KeyboardInterrupt:
        print("\nInterrupted.", file=sys.stderr)
        sys.exit(130)
    except Exception as e:
        logging.error("Review failed: %s", e)
        if args.verbose:
            raise
        sys.exit(1)


if __name__ == "__main__":
    main()
