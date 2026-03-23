from mr_reviewer.prompts import build_system_prompt


def test_build_system_prompt_default_budget():
    prompt = build_system_prompt(["bugs"])
    assert "10 inline comments maximum" in prompt


def test_build_system_prompt_custom_budget():
    prompt = build_system_prompt(["bugs"], max_comments=3)
    assert "3 inline comments maximum" in prompt
    assert "10 inline comments" not in prompt


def test_build_system_prompt_critical_exempt_language():
    prompt = build_system_prompt(["bugs"])
    assert "exempt from the budget" in prompt
