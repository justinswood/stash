import React, { useRef, useState } from "react";
import { Button, Form, InputGroup, Modal } from "react-bootstrap";
import {
  ShareType,
  useShareLinkCreateMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";

interface IProps {
  show: boolean;
  onHide: () => void;
  shareType: ShareType;
  sceneId?: string;
  imageId?: string;
  performerId?: string;
}

export const CreateShareModal: React.FC<IProps> = ({
  show,
  onHide,
  shareType,
  sceneId,
  imageId,
  performerId,
}) => {
  const Toast = useToast();
  const [createMutation, { loading }] = useShareLinkCreateMutation();

  const [expiresAt, setExpiresAt] = useState<string>("");
  const [viewLimit, setViewLimit] = useState<string>("");
  const [note, setNote] = useState<string>("");

  // Once a share is created we display its URL once. After this modal closes,
  // the token is irrecoverable.
  const [resultUrl, setResultUrl] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const urlInputRef = useRef<HTMLInputElement>(null);

  function reset() {
    setExpiresAt("");
    setViewLimit("");
    setNote("");
    setResultUrl(null);
    setCopied(false);
  }

  function handleHide() {
    reset();
    onHide();
  }

  async function handleSubmit() {
    try {
      const result = await createMutation({
        variables: {
          input: {
            share_type: shareType,
            scene_id: sceneId ?? null,
            image_id: imageId ?? null,
            performer_id: performerId ?? null,
            expires_at: expiresAt ? new Date(expiresAt).toISOString() : null,
            view_limit: viewLimit ? parseInt(viewLimit, 10) : null,
            note: note.trim() || null,
          },
        },
      });
      const url = result.data?.shareLinkCreate.url ?? null;
      if (url) {
        setResultUrl(url);
      } else {
        Toast.error(new Error("Share link created but no URL returned"));
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  async function handleCopy() {
    if (!resultUrl) return;
    // Prefer the modern API (HTTPS / localhost). Fall back to selecting the
    // text and execCommand for HTTP-on-LAN setups where navigator.clipboard
    // is undefined.
    if (navigator.clipboard?.writeText) {
      try {
        await navigator.clipboard.writeText(resultUrl);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
        return;
      } catch {
        // fall through to legacy path
      }
    }
    const input = urlInputRef.current;
    if (input) {
      input.focus();
      input.select();
      try {
        const ok = document.execCommand("copy");
        if (ok) {
          setCopied(true);
          setTimeout(() => setCopied(false), 2000);
          return;
        }
      } catch {
        // fall through
      }
    }
    Toast.success("Select the URL above and copy it manually (clipboard access is blocked over HTTP).");
  }

  return (
    <Modal show={show} onHide={handleHide} centered>
      <Modal.Header closeButton>
        <Modal.Title>Create Share Link</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        {!resultUrl ? (
          <Form>
            <Form.Group className="mb-3">
              <Form.Label>Expires at (optional)</Form.Label>
              <Form.Control
                type="datetime-local"
                value={expiresAt}
                onChange={(e) => setExpiresAt(e.target.value)}
              />
              <Form.Text className="text-muted">
                Leave empty for no expiry.
              </Form.Text>
            </Form.Group>

            <Form.Group className="mb-3">
              <Form.Label>View limit (optional)</Form.Label>
              <Form.Control
                type="number"
                min={1}
                value={viewLimit}
                onChange={(e) => setViewLimit(e.target.value)}
                placeholder="unlimited"
              />
              <Form.Text className="text-muted">
                The link stops working after this many opens.
              </Form.Text>
            </Form.Group>

            <Form.Group className="mb-3">
              <Form.Label>Note (optional)</Form.Label>
              <Form.Control
                type="text"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                placeholder="e.g. for Alice"
              />
              <Form.Text className="text-muted">
                Visible only to you, in the Share Links list.
              </Form.Text>
            </Form.Group>
          </Form>
        ) : (
          <div>
            <p>
              Share link created. <strong>Copy it now</strong> — for security,
              the URL cannot be retrieved later.
            </p>
            <InputGroup>
              <Form.Control
                ref={urlInputRef}
                type="text"
                value={resultUrl}
                readOnly
                onFocus={(e: React.FocusEvent<HTMLInputElement>) => e.currentTarget.select()}
              />
              <Button variant="secondary" onClick={handleCopy}>
                {copied ? "Copied" : "Copy"}
              </Button>
            </InputGroup>
          </div>
        )}
      </Modal.Body>
      <Modal.Footer>
        {!resultUrl ? (
          <>
            <Button variant="secondary" onClick={handleHide} disabled={loading}>
              Cancel
            </Button>
            <Button variant="primary" onClick={handleSubmit} disabled={loading}>
              {loading ? "Creating…" : "Create"}
            </Button>
          </>
        ) : (
          <Button variant="primary" onClick={handleHide}>
            Done
          </Button>
        )}
      </Modal.Footer>
    </Modal>
  );
};
