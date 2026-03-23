import argparse
import logging
import sys

from mr_reviewer.config import DEFAULT_FOCUS, DEFAULT_MODEL, Config
from mr_reviewer.core import review_mr
from mr_reviewer.exceptions import ConfigurationError, MRReviewerError
from mr_reviewer.platforms import create_platform_client
from mr_reviewer.providers import create_provider
from mr_reviewer.url_parser import parse_mr_url


def main() -> None:
    parser = argparse.ArgumentParser(
        prog="mr_reviewer",
        description="AI-powered merge request reviewer",
    )
    parser.add_argument(
        "url",
        help="GitLab MR or GitHub PR URL",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print review to stdout instead of posting",
    )
    parser.add_argument(
        "--focus",
        default=",".join(DEFAULT_FOCUS),
        help="Comma-separated review focus areas (default: bugs,style,best-practices)",
    )
    parser.add_argument(
        "--model",
        default=None,
        help=f"AI model to use (default: {DEFAULT_MODEL})",
    )
    parser.add_argument(
        "--provider",
        choices=["anthropic", "gemini", "ollama"],
        default=None,
        help="AI provider to use (default: anthropic)",
    )
    parser.add_argument(
        "--parallel",
        action="store_true",
        default=False,
        help="Enable parallel review mode (splits files across multiple agents)",
    )
    parser.add_argument(
        "--parallel-threshold",
        type=int,
        default=10,
        help="Minimum number of changed files to trigger parallel mode (default: 10)",
    )
    parser.add_argument(
        "--max-comments",
        type=int,
        default=None,
        help="Maximum number of non-critical inline comments (default: 10). "
             "Error-severity comments are always posted regardless of this limit.",
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
        config = Config()
        if args.provider:
            config.provider = args.provider
        if args.model:
            config.model = args.model

        # Auto-detect platform from URL
        mr_info = parse_mr_url(args.url)
        platform_client = create_platform_client(config, mr_info)
        provider = create_provider(config)

        max_comments = args.max_comments if args.max_comments is not None else config.max_comments

        review_mr(
            url=args.url,
            provider=provider,
            platform_client=platform_client,
            focus=focus,
            dry_run=args.dry_run,
            parallel=args.parallel or config.parallel_review,
            parallel_threshold=args.parallel_threshold,
            max_comments=max_comments,
        )
    except KeyboardInterrupt:
        print("\nInterrupted.", file=sys.stderr)
        sys.exit(130)
    except ConfigurationError as e:
        print(str(e), file=sys.stderr)
        sys.exit(1)
    except MRReviewerError as e:
        logging.error("Review failed: %s", e)
        sys.exit(1)
    except Exception as e:
        logging.error("Review failed: %s", e)
        if args.verbose:
            raise
        sys.exit(1)


if __name__ == "__main__":
    main()
