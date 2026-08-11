import { useEffect, useState } from "react";

const credentials = ["GITLAB_TOKEN", "GITHUB_TOKEN", "ANTHROPIC_API_KEY", "GEMINI_API_KEY"];
const settings = ["MR_REVIEWER_GITLAB_URL", "OLLAMA_HOST", "MR_REVIEWER_PROVIDER", "MR_REVIEWER_MODEL"];

export function SettingsPage() {
  const electronAPI = window.electronAPI;
  const [values, setValues] = useState<Record<string, string>>({});
  const [present, setPresent] = useState<Record<string, boolean>>({});
  const [message, setMessage] = useState("");
  useEffect(() => { electronAPI?.getRuntimeSettings().then((data) => { setValues(data.settings); setPresent(data.credentials); }); }, [electronAPI]);
  if (!electronAPI) return <p className="text-sm text-muted-foreground">Settings are managed through environment variables in web mode.</p>;
  const save = async () => { await electronAPI.saveRuntimeSettings(values); setMessage("Review defaults saved. Restart the desktop backend to apply them."); };
  return <div className="max-w-xl space-y-6"><div><h1 className="text-lg font-medium">Settings</h1><p className="text-sm text-muted-foreground">Credentials are stored in the OS keychain. Review defaults are stored locally.</p></div><section className="space-y-3"><h2 className="font-medium">Credentials</h2>{credentials.map((key) => <div key={key} className="flex gap-2"><input type="password" placeholder={`${key} ${present[key] ? "configured" : "not configured"}`} onChange={(event) => setValues((old) => ({ ...old, [key]: event.target.value }))} className="h-9 flex-1 rounded border border-border bg-surface px-3"/><button onClick={() => electronAPI.saveCredential(key, values[key] || "").then(() => setMessage(`${key} saved`))}>Save</button></div>)}</section><section className="space-y-3"><h2 className="font-medium">Review defaults</h2>{settings.map((key) => <label key={key} className="block text-sm">{key}<input value={values[key] || ""} onChange={(event) => setValues((old) => ({ ...old, [key]: event.target.value }))} className="mt-1 h-9 w-full rounded border border-border bg-surface px-3"/></label>)}<button onClick={save}>Save defaults</button></section>{message && <p className="text-sm text-success">{message}</p>}</div>;
}
