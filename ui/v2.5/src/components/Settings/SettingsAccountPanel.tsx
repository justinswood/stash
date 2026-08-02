import React, { useState } from "react";
import { Button, Form } from "react-bootstrap";
import {
  UserRole,
  useMeQuery,
  useChangePasswordMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";
import { UserAPIKeys } from "./UserAPIKeys";

const roleLabel: Record<UserRole, string> = {
  [UserRole.Admin]: "Admin",
  [UserRole.User]: "User",
  [UserRole.ReadOnly]: "Read-only",
};

export const SettingsAccountPanel: React.FC = () => {
  const Toast = useToast();
  const { data: meData } = useMeQuery();
  const [changePassword] = useChangePasswordMutation();

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirm, setConfirm] = useState("");
  const [saving, setSaving] = useState(false);

  const me = meData?.me;

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (next !== confirm) {
      Toast.error(new Error("New passwords do not match"));
      return;
    }
    setSaving(true);
    try {
      await changePassword({
        variables: {
          input: { current_password: current, new_password: next },
        },
      });
      Toast.success("Password changed");
      setCurrent("");
      setNext("");
      setConfirm("");
    } catch (err) {
      Toast.error(err);
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <h4>Account</h4>
      {me ? (
        <p>
          Signed in as <strong>{me.username}</strong> ({roleLabel[me.role]})
        </p>
      ) : (
        <p>No account — this system has no login configured.</p>
      )}

      {me && (
        <Form onSubmit={onSubmit} style={{ maxWidth: 420 }}>
          <h5 className="mt-4">Change password</h5>
          <Form.Group className="mb-3">
            <Form.Label>Current password</Form.Label>
            <Form.Control
              type="password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              autoComplete="current-password"
            />
          </Form.Group>
          <Form.Group className="mb-3">
            <Form.Label>New password</Form.Label>
            <Form.Control
              type="password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              autoComplete="new-password"
            />
          </Form.Group>
          <Form.Group className="mb-3">
            <Form.Label>Confirm new password</Form.Label>
            <Form.Control
              type="password"
              value={confirm}
              onChange={(e) => setConfirm(e.target.value)}
              autoComplete="new-password"
            />
          </Form.Group>
          <Button
            type="submit"
            variant="primary"
            disabled={saving || !current || !next}
          >
            Change password
          </Button>
        </Form>
      )}

      {me && <UserAPIKeys />}
    </>
  );
};
