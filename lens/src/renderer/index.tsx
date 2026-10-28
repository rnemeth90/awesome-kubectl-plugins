import { Renderer } from "@freelensapp/extensions";
import { type ReactNode, useState } from "react";
import { type Field, type Tool, type Values, categories, tools } from "../common/tools";

const { Button, Input, Checkbox, Spinner } = Renderer.Component;

interface Result {
  ok: boolean;
  output: string;
}

function FieldInput({ field, value, onChange }: { field: Field; value: Values[string]; onChange: (v: string | boolean) => void }) {
  if (field.kind === "boolean") {
    return <Checkbox label={field.label} value={value === true} onChange={(v) => onChange(v)} />;
  }
  return (
    <div style={{ marginBottom: 12, maxWidth: 480 }}>
      <label style={{ display: "block", marginBottom: 4 }}>
        {field.label}
        {field.required ? " *" : ""}
      </label>
      <Input
        type={field.kind === "number" ? "number" : "text"}
        placeholder={field.placeholder}
        value={String(value ?? "")}
        onChange={(v: string) => onChange(v)}
      />
    </div>
  );
}

function ToolPage({ tool }: { tool: Tool }): ReactNode {
  const [values, setValues] = useState<Values>({});
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState<Result | null>(null);
  const [confirming, setConfirming] = useState(false);

  const main = tool.fields.filter((f) => !f.advanced);
  const advanced = tool.fields.filter((f) => f.advanced);
  const set = (name: string) => (v: string | boolean) => setValues((prev) => ({ ...prev, [name]: v }));

  const run = async () => {
    setConfirming(false);
    const cluster = Renderer.Catalog.getActiveCluster();
    if (!cluster) {
      setResult({ ok: false, output: "No active cluster." });
      return;
    }
    setBusy(true);
    setResult(null);
    try {
      const res = (await PluginsIpc.getInstance().invoke("run", { clusterId: cluster.id, toolId: tool.id, values })) as Result;
      setResult(res);
    } catch (e) {
      setResult({ ok: false, output: String(e) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{ padding: 24, overflow: "auto", height: "100%" }}>
      <h2>{tool.title}</h2>
      <p style={{ opacity: 0.8, marginBottom: 16 }}>
        {tool.description} Runs <code>{tool.binary}</code> against the current cluster.
      </p>
      {main.map((f) => (
        <FieldInput key={f.name} field={f} value={values[f.name]} onChange={set(f.name)} />
      ))}
      {advanced.length > 0 && (
        <details style={{ marginBottom: 12 }}>
          <summary>Advanced (override auto-detected cluster)</summary>
          <div style={{ marginTop: 8 }}>
            {advanced.map((f) => (
              <FieldInput key={f.name} field={f} value={values[f.name]} onChange={set(f.name)} />
            ))}
          </div>
        </details>
      )}
      {tool.mutating && confirming ? (
        <div>
          <p style={{ marginBottom: 8, color: "var(--colorError)" }}>
            This changes resources in your cluster or Azure subscription. Run it?
          </p>
          <Button primary label="Confirm and run" onClick={run} />{" "}
          <Button plain label="Cancel" onClick={() => setConfirming(false)} />
        </div>
      ) : (
        <Button
          primary
          label={tool.mutating ? "Run…" : "Run"}
          waiting={busy}
          disabled={busy}
          onClick={tool.mutating ? () => setConfirming(true) : run}
        />
      )}
      {busy && <Spinner />}
      {result && (
        <pre
          style={{
            marginTop: 16,
            padding: 12,
            whiteSpace: "pre-wrap",
            userSelect: "text",
            border: `1px solid ${result.ok ? "var(--borderColor)" : "var(--colorError)"}`,
            borderRadius: 4,
          }}
        >
          {result.output || "(no output)"}
        </pre>
      )}
    </div>
  );
}

class PluginsIpc extends Renderer.Ipc {}

const Icon = () => <Renderer.Component.Icon material="extension" />;

export default class KubectlPluginsRenderer extends Renderer.LensExtension {
  async onActivate() {
    PluginsIpc.createInstance(this);
  }

  clusterPages = tools.map((t) => ({
    id: t.id,
    components: { Page: () => <ToolPage tool={t} /> },
  }));

  clusterPageMenus = [
    { id: "kubectl-plugins", title: "kubectl plugins", target: { pageId: tools[0].id }, components: { Icon } },
    ...categories.flatMap((c) =>
      tools
        .filter((t) => t.category === c)
        .map((t) => ({
          id: `menu-${t.id}`,
          parentId: "kubectl-plugins",
          title: t.title,
          target: { pageId: t.id },
          components: { Icon: () => null },
        })),
    ),
  ];
}
