import React, { useState } from "react";
import { Button, Form, Table } from "react-bootstrap";
import { IndeterminateCheckbox } from "../Shared/IndeterminateCheckbox";
import {
  ShareLinkDataFragment,
  ShareType,
  useFindShareLinksQuery,
  useShareLinkDestroyMutation,
  useShareLinkRevokeMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

function fmtDate(s?: string | null) {
  if (!s) return "—";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return d.toLocaleString();
}

function targetLink(link: ShareLinkDataFragment): React.ReactNode {
  switch (link.share_type) {
    case ShareType.Scene:
      return link.scene_id ? (
        <a href={`/scenes/${link.scene_id}`}>Scene #{link.scene_id}</a>
      ) : (
        "—"
      );
    case ShareType.Image:
      return link.image_id ? (
        <a href={`/images/${link.image_id}`}>Image #{link.image_id}</a>
      ) : (
        "—"
      );
    case ShareType.Performer:
      return link.performer_id ? (
        <a href={`/performers/${link.performer_id}`}>
          Performer #{link.performer_id}
        </a>
      ) : (
        "—"
      );
    default:
      return "—";
  }
}

function statusLabel(link: ShareLinkDataFragment): string {
  if (link.revoked) return "Revoked";
  if (link.expires_at && new Date(link.expires_at) < new Date()) {
    return "Expired";
  }
  if (link.view_limit != null && link.view_count >= link.view_limit) {
    return "Used up";
  }
  return "Active";
}

export const SettingsShareLinksPanel: React.FC = () => {
  const Toast = useToast();
  const { data, loading, refetch } = useFindShareLinksQuery({
    fetchPolicy: "cache-and-network",
  });
  const [revokeMutation] = useShareLinkRevokeMutation();
  const [destroyMutation] = useShareLinkDestroyMutation();
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [bulkDeleting, setBulkDeleting] = useState(false);

  async function handleRevoke(id: string) {
    try {
      await revokeMutation({ variables: { id } });
      await refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function handleDestroy(id: string) {
    if (!window.confirm("Permanently delete this share link?")) return;
    try {
      await destroyMutation({ variables: { id } });
      await refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  const links = data?.findShareLinks ?? [];

  // Derived from the current rows rather than read straight off selectedIds, so
  // ids left over from links that have since been deleted elsewhere can't inflate
  // the count or wrongly satisfy the "all selected" check.
  const selected = links.filter((link) => selectedIds.has(link.id));
  const allSelected =
    selected.length === 0
      ? false
      : selected.length === links.length
      ? true
      : undefined;

  function toggleOne(id: string, checked: boolean) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (checked) next.add(id);
      else next.delete(id);
      return next;
    });
  }

  function toggleAll(checked: boolean | undefined) {
    setSelectedIds(checked ? new Set(links.map((link) => link.id)) : new Set());
  }

  async function handleDestroySelected() {
    const ids = selected.map((link) => link.id);
    if (ids.length === 0) return;
    if (
      !window.confirm(
        `Permanently delete ${ids.length} share link${
          ids.length === 1 ? "" : "s"
        }? This cannot be undone.`
      )
    )
      return;

    // No bulk mutation exists server-side, so this walks the single-id one.
    // Sequential rather than parallel: it keeps a partial failure attributable
    // and avoids firing N concurrent mutations from a settings screen.
    setBulkDeleting(true);
    let failures = 0;
    for (const id of ids) {
      try {
        await destroyMutation({ variables: { id } });
      } catch (e) {
        failures += 1;
      }
    }
    setBulkDeleting(false);
    setSelectedIds(new Set());
    await refetch();

    if (failures > 0) {
      Toast.error(
        new Error(`Failed to delete ${failures} of ${ids.length} share links.`)
      );
    }
  }

  return (
    <div className="settings-section">
      <h1>Share Links</h1>
      <p className="text-muted">
        Tokenized links you've created to share content with people who don't
        have access to Stash. Revoke a link to invalidate it immediately;
        delete it to remove it from this list.
      </p>
      {loading && links.length === 0 ? (
        <p>Loading…</p>
      ) : links.length === 0 ? (
        <p>No share links yet. Use the &ldquo;Share&rdquo; action on a scene to create one.</p>
      ) : (
        <>
          <div className="d-flex align-items-center mb-2">
            <Button
              size="sm"
              variant="danger"
              disabled={selected.length === 0 || bulkDeleting}
              onClick={handleDestroySelected}
            >
              {bulkDeleting
                ? "Deleting…"
                : `Delete Selected (${selected.length})`}
            </Button>
          </div>
          <Table size="sm" striped variant="dark">
            <thead>
              <tr>
                <th>
                  <IndeterminateCheckbox
                    checked={allSelected}
                    setChecked={toggleAll}
                    allowIndeterminate={false}
                    aria-label="Select all share links"
                  />
                </th>
                <th>Status</th>
                <th>Type</th>
                <th>Target</th>
                <th>Note</th>
                <th>Created</th>
                <th>Expires</th>
                <th>Views</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {links.map((link) => (
                <tr key={link.id}>
                  <td>
                    <Form.Check
                      checked={selectedIds.has(link.id)}
                      onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                        toggleOne(link.id, e.currentTarget.checked)
                      }
                      aria-label={`Select share link ${link.id}`}
                    />
                  </td>
                  <td>{statusLabel(link)}</td>
                  <td>{link.share_type}</td>
                  <td>{targetLink(link)}</td>
                  <td>{link.note ?? "—"}</td>
                  <td>{fmtDate(link.created_at)}</td>
                  <td>{fmtDate(link.expires_at)}</td>
                  <td>
                    {link.view_count}
                    {link.view_limit != null ? ` / ${link.view_limit}` : ""}
                  </td>
                  <td>
                    {!link.revoked && (
                      <Button
                        size="sm"
                        variant="warning"
                        className="mr-2"
                        disabled={bulkDeleting}
                        onClick={() => handleRevoke(link.id)}
                      >
                        Revoke
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="danger"
                      disabled={bulkDeleting}
                      onClick={() => handleDestroy(link.id)}
                    >
                      Delete
                    </Button>
                  </td>
                </tr>
              ))}
            </tbody>
          </Table>
        </>
      )}
    </div>
  );
};
