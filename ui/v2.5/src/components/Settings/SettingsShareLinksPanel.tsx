import React from "react";
import { Button, Table } from "react-bootstrap";
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
        <Table size="sm" striped variant="dark">
          <thead>
            <tr>
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
                      onClick={() => handleRevoke(link.id)}
                    >
                      Revoke
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => handleDestroy(link.id)}
                  >
                    Delete
                  </Button>
                </td>
              </tr>
            ))}
          </tbody>
        </Table>
      )}
    </div>
  );
};
