from mr_reviewer.diff_parser import get_changed_file_paths, parse_diff, validate_comment_line


DIFF_ADDITIONS_ONLY = """\
--- a/hello.py
+++ b/hello.py
@@ -1,3 +1,5 @@
 import os
+import sys
+import json

 def main():
"""

DIFF_DELETIONS_ONLY = """\
--- a/hello.py
+++ b/hello.py
@@ -1,5 +1,3 @@
 import os
-import sys
-import json

 def main():
"""

DIFF_MIXED = """\
--- a/hello.py
+++ b/hello.py
@@ -1,4 +1,4 @@
 import os
-import sys
+import json

 def main():
"""

DIFF_MULTI_FILE = """\
--- a/foo.py
+++ b/foo.py
@@ -1,3 +1,4 @@
 import os
+import sys

 def foo():
--- a/bar.py
+++ b/bar.py
@@ -1,3 +1,4 @@
 import json
+import csv

 def bar():
"""


def test_parse_diff_additions_only():
    lines = parse_diff(DIFF_ADDITIONS_ONLY)
    assert len(lines) == 2
    assert all(dl.line_type == "+" for dl in lines)
    assert lines[0].new_line == 2
    assert lines[1].new_line == 3


def test_parse_diff_deletions_only():
    lines = parse_diff(DIFF_DELETIONS_ONLY)
    assert len(lines) == 2
    assert all(dl.line_type == "-" for dl in lines)
    assert lines[0].old_line == 2
    assert lines[1].old_line == 3


def test_parse_diff_mixed():
    lines = parse_diff(DIFF_MIXED)
    assert len(lines) == 2
    types = [dl.line_type for dl in lines]
    assert "-" in types
    assert "+" in types


def test_parse_diff_empty_string():
    lines = parse_diff("")
    assert lines == []


def test_parse_diff_invalid_string():
    lines = parse_diff("this is not a diff")
    assert lines == []


def test_validate_comment_line_positive_new_line():
    lines = parse_diff(DIFF_ADDITIONS_ONLY)
    assert validate_comment_line("hello.py", 2, lines) is True


def test_validate_comment_line_positive_old_line():
    lines = parse_diff(DIFF_DELETIONS_ONLY)
    assert validate_comment_line("hello.py", 2, lines) is True


def test_validate_comment_line_negative():
    lines = parse_diff(DIFF_ADDITIONS_ONLY)
    assert validate_comment_line("hello.py", 999, lines) is False


def test_get_changed_file_paths_multiple_files():
    paths = get_changed_file_paths(DIFF_MULTI_FILE)
    assert len(paths) == 2
    assert "foo.py" in paths
    assert "bar.py" in paths
