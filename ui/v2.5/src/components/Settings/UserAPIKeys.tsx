import React, { useRef, useState } from "react";
import { Alert, Button, Form, Table } from "react-bootstrap";
import {
  useFindUserApiKeysQuery,
  useUserApiKeyCreateMutation,
  useUserApiKeyRevokeMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

function formatDate(value?: string | null) {
  if (!value) return "Never";
  return new Date(value).toLocaleString();
}

export const UserAPIKeys: React.FC = () => {
  const Toast = useToast();
  const { data, loading, refetch } = useFindUserApiKeysQuery({
    variables: {},
    fetchPolicy: "network-only",
  });
  const [createKey] = useUserApiKeyCreateMutation();
  const [revokeKey] = useUserApiKeyRevokeMutation();

  const [name, setName] = useState("");
  const [creating, setCreating] = useState(false);
  // The plaintext is returned exactly once, so it lives in component state
  // until the user dismisses it. There is no way to show it again.
  const [newKey, setNewKey] = useState<string | undefined>();
  const [copied, setCopied] = useState(false);
  const keyInputRef = useRef<HTMLInputElement>(null);

  const keys = data?.findUserAPIKeys ?? [];

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!name.trim()) return;
    setCreating(true);
    try {
      const result = await createKey({ variables: { input: { name } } });
      setNewKey(result.data?.userAPIKeyCreate.key);
      setName("");
      await refetch();
    } catch (err) {
      Toast.error(err);
    } finally {
      setCreating(false);
    }
  }

  async function onRevoke(id: string, keyName: string) {
    if (
      !window.confirm(
        `Revoke "${keyName}"? Any client using this key will stop working immediately.`
      )
    ) {
      return;
    }
    try {
      await revokeKey({ variables: { id } });
      Toast.success("API key revoked");
      await refetch();
    } catch (err) {
      Toast.error(err);
    }
  }

  async function onCopy() {
    if (!newKey) return;
    // Prefer the modern API (HTTPS / localhost). Fall back to selecting the
    // text and execCommand for HTTP-on-LAN setups where navigator.clipboard
    // is undefined.
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(newKey);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
        return;
      } catch {
        // fall through to legacy path
      }
    }
    const input = keyInputRef.current;
    if (input) {
      input.focus();
      input.select();
      try {
        if (document.execCommand("copy")) {
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
          return;
        }
      } catch {
        // fall through
      }
    }
    Toast.error(new Error("Could not copy — select the key and copy manually"));
  }

  return (
    <>
      <h5 className="mt-4">API keys</h5>
      <p className="text-muted">
        A key acts as your account and carries your role. Revoke a key here, or
        disable the account, to stop it working.
      </p>

      {newKey && (
        <Alert variant="success" onClose={() => setNewKey(undefined)} dismissible>
          <Alert.Heading>Copy your new key now</Alert.Heading>
          <p>This is the only time it will be shown. It cannot be recovered.</p>
          <Form.Control
            ref={keyInputRef}
            readOnly
            value={newKey}
            onFocus={(e: React.FocusEvent<HTMLInputElement>) =>
              e.currentTarget.select()
            }
          />
          <Button className="mt-2" variant="secondary" onClick={onCopy}>
            {copied ? "Copied" : "Copy"}
          </Button>
        </Alert>
      )}

      <Form onSubmit={onCreate} className="mb-3" style={{ maxWidth: 420 }}>
        <Form.Group className="mb-2">
          <Form.Label>New key name</Form.Label>
          <Form.Control
            value={name}
            placeholder="e.g. scraper on media-box"
            onChange={(e) => setName(e.target.value)}
          />
        </Form.Group>
        <Button type="submit" variant="primary" disabled={creating || !name.trim()}>
          Create API key
        </Button>
      </Form>

      {loading ? (
        <p>Loading…</p>
      ) : keys.length === 0 ? (
        <p className="text-muted">No API keys.</p>
      ) : (
        <Table responsive size="sm">
          <thead>
            <tr>
              <th>Name</th>
              <th>Key</th>
              <th>Last used</th>
              <th>Created</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {keys.map((k) => (
              <tr key={k.id}>
                <td>{k.name}</td>
                <td>
                  <code>{k.prefix}…</code>
                </td>
                <td>{formatDate(k.last_used_at)}</td>
                <td>{formatDate(k.created_at)}</td>
                <td>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => onRevoke(k.id, k.name)}
                  >
                    Revoke
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </>
  );
};
