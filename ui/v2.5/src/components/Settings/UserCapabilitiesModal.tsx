import React, { useMemo, useState } from "react";
import { Button, Form, Modal, Spinner } from "react-bootstrap";
import {
  Capability,
  UserRole,
  useAllCapabilitiesQuery,
  useFindUserCapabilitiesQuery,
  useUserCapabilitiesSetMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

interface IProps {
  userId: string;
  username: string;
  onClose: () => void;
  onSaved: () => void;
}

// Short human labels. The schema carries the authoritative descriptions; these
// are just what fits in a checkbox row.
const capabilityLabel: Partial<Record<Capability, string>> = {
  [Capability.ViewLibrary]: "Browse the library",
  [Capability.OwnHistory]: "Record own watch history",
  [Capability.ChangeOwnPassword]: "Change own password",
  [Capability.EditMetadata]: "Edit metadata",
  [Capability.Scrape]: "Run scrapers",
  [Capability.ManageShares]: "Create and revoke share links",
  [Capability.ManageOwnApiKeys]: "Manage own API keys",
  [Capability.DeleteContent]: "Delete content and files",
  [Capability.ManageUsers]: "Manage user accounts",
  [Capability.Configure]: "Change configuration",
  [Capability.RunTasks]: "Run library tasks",
  [Capability.ViewSystem]: "View logs and job queue",
  [Capability.BrowseFilesystem]: "Browse the host filesystem",
  [Capability.ManagePlugins]: "Manage plugins and packages",
  [Capability.ExecuteSql]: "Run arbitrary SQL",
};

export const UserCapabilitiesModal: React.FC<IProps> = ({
  userId,
  username,
  onClose,
  onSaved,
}) => {
  const Toast = useToast();
  const { data: allData } = useAllCapabilitiesQuery();
  const { data: userData, loading } = useFindUserCapabilitiesQuery({
    variables: { id: userId },
    fetchPolicy: "network-only",
  });
  const [setCapabilities] = useUserCapabilitiesSetMutation();

  const [saving, setSaving] = useState(false);
  // capability -> checked. Seeded from the server's effective set once loaded.
  const [checked, setChecked] = useState<Record<string, boolean> | undefined>();

  const all = allData?.allCapabilities ?? [];
  const user = userData?.findUser;

  const effective = useMemo(
    () => new Set(user?.capabilities ?? []),
    [user?.capabilities]
  );

  // The preset for this user's role, derived by subtracting the stored
  // overrides from the effective set — the server owns the preset definition,
  // so we never hardcode it here.
  const preset = useMemo(() => {
    const p = new Set(effective);
    for (const o of user?.capability_overrides ?? []) {
      if (o.granted) p.delete(o.capability);
      else p.add(o.capability);
    }
    return p;
  }, [effective, user?.capability_overrides]);

  const state = checked ?? Object.fromEntries(all.map((c) => [c, effective.has(c)]));

  function toggle(cap: string) {
    setChecked({ ...state, [cap]: !state[cap] });
  }

  function resetToRoleDefaults() {
    setChecked(Object.fromEntries(all.map((c) => [c, preset.has(c)])));
  }

  async function onSave() {
    setSaving(true);
    try {
      // Send only departures from the preset; the server drops redundant ones
      // anyway, but this keeps the stored set minimal and the intent explicit.
      const overrides = all
        .filter((c) => state[c] !== preset.has(c))
        .map((c) => ({ capability: c, granted: state[c] }));

      await setCapabilities({
        variables: { input: { user_id: userId, overrides } },
      });
      Toast.success("Permissions updated");
      onSaved();
      onClose();
    } catch (err) {
      Toast.error(err);
    } finally {
      setSaving(false);
    }
  }

  const overrideCount = all.filter((c) => state[c] !== preset.has(c)).length;

  return (
    <Modal show onHide={onClose} size="lg">
      <Modal.Header closeButton>
        <Modal.Title>Permissions — {username}</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        {loading || !user ? (
          <Spinner animation="border" role="status" />
        ) : (
          <>
            <p className="text-muted">
              Role <strong>{user.role}</strong> sets the defaults below. Ticking or
              unticking a box overrides the role for this account only; changing
              the role later re-applies its defaults to anything not overridden.
            </p>
            {all.map((cap) => {
              const isOverride = state[cap] !== preset.has(cap);
              return (
                <Form.Check
                  key={cap}
                  type="checkbox"
                  id={`cap-${cap}`}
                  checked={!!state[cap]}
                  onChange={() => toggle(cap)}
                  label={
                    <>
                      {capabilityLabel[cap as Capability] ?? cap}{" "}
                      <code className="text-muted">{cap}</code>
                      {isOverride && (
                        <span className="ml-2 badge badge-warning">
                          {state[cap] ? "granted" : "revoked"}
                        </span>
                      )}
                    </>
                  }
                />
              );
            })}
            <p className="text-muted mt-3">
              {overrideCount === 0
                ? "No overrides — this account uses its role defaults."
                : `${overrideCount} override${overrideCount === 1 ? "" : "s"}.`}
            </p>
          </>
        )}
      </Modal.Body>
      <Modal.Footer>
        <Button
          variant="secondary"
          onClick={resetToRoleDefaults}
          disabled={saving || loading || overrideCount === 0}
        >
          Reset to role defaults
        </Button>
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button variant="primary" onClick={onSave} disabled={saving || loading}>
          Save
        </Button>
      </Modal.Footer>
    </Modal>
  );
};

// Re-exported so the users panel can label rows without duplicating the map.
export { capabilityLabel };
export type { UserRole };
