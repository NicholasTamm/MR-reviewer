class MRReviewerError(Exception):
    """Base exception for all mr_reviewer errors."""


class ConfigurationError(MRReviewerError):
    """Missing tokens, invalid config."""


class ProviderError(MRReviewerError):
    """AI provider API failures."""


class PlatformError(MRReviewerError):
    """VCS platform API failures."""


class ReviewError(MRReviewerError):
    """Review orchestration failures."""
