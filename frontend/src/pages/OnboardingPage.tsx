import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import {
  cancelAuthSession,
  completeOnboarding,
  getAuthSession,
  getOnboarding,
  saveOnboardingSecret,
  startAuthSession,
} from "@/lib/api";
import type { AuthSession, OnboardingOption, OnboardingStatus } from "@/types/api";

export function OnboardingPage() {
  const navigate = useNavigate();
  const [status, setStatus] = useState<OnboardingStatus | null>(null);
  const [step, setStep] = useState<"provider" | "platform" | "secret" | "auth">("provider");
  const [kind, setKind] = useState<"provider" | "platform">("provider");
  const [selected, setSelected] = useState("");
  const [secret, setSecret] = useState("");
  const [error, setError] = useState("");
  const [session, setSession] = useState<AuthSession | null>(null);

  const refresh = () => getOnboarding().then(setStatus).catch((err: Error) => setError(err.message));
  useEffect(() => { refresh(); }, []);
  useEffect(() => {
    if (!session || (session.status !== "pending" && session.status !== "failed")) return;
    if (session.status === "failed") return;
    const timer = setInterval(() => {
      getAuthSession(session.session_id).then((next) => {
        setSession(next);
        if (next.status === "complete") {
          setSession(null);
          if (kind === "provider") setStep("platform");
          else finish(status?.selected_provider || selectedProvider(status), selected);
        }
      }).catch((err: Error) => setError(err.message));
    }, 1000);
    return () => clearInterval(timer);
  }, [session?.session_id, session?.status]); // eslint-disable-line react-hooks/exhaustive-deps

  if (!status) return <p className="text-sm text-muted-foreground">{error || "Checking shared setup..."}</p>;
  if (status.complete) {
    navigate("/");
    return null;
  }

  const options = step === "platform" || (step !== "provider" && kind === "platform") ? status.platforms : status.providers;
  const current = options.find((item) => item.id === selected);

  return (
    <div className="mx-auto max-w-xl space-y-5">
      <div>
        <p className="text-xs font-mono uppercase tracking-widest text-muted-foreground">{status.repair ? "Repair setup" : "First-run setup"}</p>
        <h1 className="mt-1 text-xl font-medium">Configure one AI provider and one Git platform</h1>
        <p className="mt-2 text-sm text-muted-foreground">{status.reason || "This uses the same ~/.mr-reviewer store as the TUI."}</p>
      </div>
      {error && <p className="text-sm text-destructive">{error}</p>}

      {(step === "provider" || step === "platform") && (
        <OptionList
          label={step === "provider" ? "AI provider" : "Git platform"}
          options={options}
          onPick={(option) => {
            setSelected(option.id);
            setError("");
            if (option.has_credential) {
              if (step === "provider") setStep("platform");
              else finish(status.selected_provider || selectedProvider(status), option.id);
              return;
            }
            setKind(step === "provider" ? "provider" : "platform");
            setStep("secret");
          }}
        />
      )}

      {step === "secret" && current && (
        <div className="space-y-3 rounded-lg border border-border bg-surface p-4">
          <p className="text-sm">How do you want to authenticate {current.id}?</p>
          <div className="flex flex-wrap gap-2">
            {current.methods.includes("key") && <button className="rounded-md border border-border px-3 py-2 text-sm" onClick={() => setStep("secret")}>API key / PAT</button>}
            {current.methods.filter((method) => method !== "key").map((method) => (
              <button key={method} className="rounded-md border border-primary/40 px-3 py-2 text-sm text-primary" onClick={() => startAuth(kind, current.id, method)}>{method === "device" ? "Device OAuth" : "OAuth"}</button>
            ))}
          </div>
          <input type="password" value={secret} onChange={(event) => setSecret(event.target.value)} placeholder={kind === "platform" ? "personal access token" : "API key"} className="h-10 w-full rounded-md border border-border bg-background px-3" />
          <div className="flex gap-2">
            <button className="rounded-md bg-primary px-3 py-2 text-sm text-primary-foreground" onClick={() => saveSecret(kind, current.id)}>Save</button>
            <button className="rounded-md border border-border px-3 py-2 text-sm" onClick={() => { setSecret(""); setStep(kind); }}>Back</button>
          </div>
        </div>
      )}

      {session && (
        <div className="space-y-3 rounded-lg border border-primary/40 bg-surface p-4">
          <h2 className="font-medium">{session.name.toUpperCase()} authorization {session.status === "failed" ? "failed" : "in progress"}</h2>
          {session.verification_uri && <p className="text-sm">1. Open this URL: <span className="font-mono">{session.verification_uri_complete || session.verification_uri}</span></p>}
          {session.user_code && <p className="text-lg font-mono tracking-widest">{session.user_code}</p>}
          {session.authorization_url && <p className="text-sm">Complete authorization in your browser: <span className="font-mono">{session.authorization_url}</span></p>}
          {session.error && <p className="text-sm text-destructive">{session.error}</p>}
          <div className="flex gap-2">
            {session.status === "failed" && <button className="rounded-md border border-primary/40 px-3 py-2 text-sm" onClick={() => startAuth(session.kind as "provider" | "platform", session.name, session.method)}>Retry</button>}
            <button className="rounded-md border border-border px-3 py-2 text-sm" onClick={() => { if (session.session_id) cancelAuthSession(session.session_id); setSession(null); setStep(kind); }}>Cancel</button>
          </div>
        </div>
      )}
    </div>
  );

  function selectedProvider(current: OnboardingStatus | null) {
    return current?.selected_provider || current?.providers.find((item) => item.has_credential)?.id || selected;
  }

  async function saveSecret(nextKind: "provider" | "platform", name: string) {
    try {
      await saveOnboardingSecret(nextKind, name, secret);
      setSecret("");
      if (nextKind === "provider") setStep("platform");
      else await finish(selectedProvider(status), name);
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function startAuth(nextKind: "provider" | "platform", name: string, method: string) {
    try {
      setKind(nextKind);
      const next = await startAuthSession(nextKind, name, method);
      setSession(next);
      setStep("auth");
    } catch (err) {
      setError((err as Error).message);
    }
  }

  async function finish(provider: string, platform: string) {
    try {
      const next = await completeOnboarding(provider, platform);
      setStatus(next);
      if (next.complete) navigate("/");
    } catch (err) {
      setError((err as Error).message);
    }
  }
}

function OptionList({ label, options, onPick }: { label: string; options: OnboardingOption[]; onPick: (option: OnboardingOption) => void }) {
  return (
    <section className="space-y-2">
      <h2 className="text-sm font-medium">{label}</h2>
      {options.map((option) => (
        <button key={option.id} onClick={() => onPick(option)} className="flex w-full items-center justify-between rounded-md border border-border bg-surface px-4 py-3 text-left hover:bg-muted">
          <span className="font-mono">{option.id}</span>
          <span className="text-xs text-muted-foreground">{option.has_credential ? "credential ready" : option.methods.join(" / ")}</span>
        </button>
      ))}
    </section>
  );
}
