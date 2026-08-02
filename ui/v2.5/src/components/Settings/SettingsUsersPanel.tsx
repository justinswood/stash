import React, { useState } from "react";
import { Button, Form, Modal, Table } from "react-bootstrap";
import {
  UserDataFragment,
  UserRole,
  useFindUsersQuery,
  useMeQuery,
  useUserCreateMutation,
  useUserUpdateMutation,
  useUserDestroyMutation,
} from "src/core/generated-graphql";
import { useToast } from "src/hooks/Toast";
import { UserCapabilitiesModal } from "./UserCapabilitiesModal";

const roleLabel: Record<UserRole, string> = {
  [UserRole.Admin]: "Admin",
  [UserRole.User]: "User",
  [UserRole.ReadOnly]: "Read-only",
};

interface IUserModalProps {
  user?: UserDataFragment; // undefined = create
  onClose: () => void;
  onSaved: () => void;
}

const UserModal: React.FC<IUserModalProps> = ({ user, onClose, onSaved }) => {
  const Toast = useToast();
  const editing = !!user;
  const [username, setUsername] = useState(user?.username ?? "");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState<UserRole>(user?.role ?? UserRole.User);
  const [disabled, setDisabled] = useState<boolean>(user?.disabled ?? false);
  const [saving, setSaving] = useState(false);

  const [createUser] = useUserCreateMutation();
  const [updateUser] = useUserUpdateMutation();

  async function onSave() {
    setSaving(true);
    try {
      if (editing) {
        await updateUser({
          variables: {
            input: {
              id: user!.id,
              username,
              role,
              disabled,
              ...(password ? { password } : {}),
            },
          },
        });
      } else {
        await createUser({
          variables: { input: { username, password, role, disabled } },
        });
      }
      onSaved();
      onClose();
    } catch (e) {
      Toast.error(e);
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal show onHide={onClose}>
      <Modal.Header closeButton>
        <Modal.Title>{editing ? "Edit User" : "Add User"}</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <Form.Group className="mb-3">
          <Form.Label>Username</Form.Label>
          <Form.Control
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus={!editing}
          />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>
            {editing ? "New password (leave blank to keep)" : "Password"}
          </Form.Label>
          <Form.Control
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="new-password"
          />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>Role</Form.Label>
          <Form.Control
            as="select"
            value={role}
            onChange={(e) => setRole(e.target.value as UserRole)}
          >
            <option value={UserRole.Admin}>{roleLabel[UserRole.Admin]}</option>
            <option value={UserRole.User}>{roleLabel[UserRole.User]}</option>
            <option value={UserRole.ReadOnly}>
              {roleLabel[UserRole.ReadOnly]}
            </option>
          </Form.Control>
        </Form.Group>
        <Form.Check
          id="user-disabled"
          label="Disabled (cannot log in)"
          checked={disabled}
          onChange={(e) => setDisabled(e.target.checked)}
        />
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={onSave}
          disabled={saving || !username || (!editing && !password)}
        >
          Save
        </Button>
      </Modal.Footer>
    </Modal>
  );
};

export const SettingsUsersPanel: React.FC = () => {
  const Toast = useToast();
  const { data: meData } = useMeQuery();
  const { data, loading, refetch } = useFindUsersQuery();
  const [destroyUser] = useUserDestroyMutation();
  const [modalUser, setModalUser] = useState<UserDataFragment | null>(null);
  const [capsUser, setCapsUser] = useState<UserDataFragment | null>(null);
  const [showCreate, setShowCreate] = useState(false);

  const isAdmin = !meData?.me || meData.me.role === UserRole.Admin;

  if (!isAdmin) {
    return <p>You must be an administrator to manage users.</p>;
  }

  async function onDelete(u: UserDataFragment) {
    if (!window.confirm(`Delete user "${u.username}"? This cannot be undone.`)) {
      return;
    }
    try {
      await destroyUser({ variables: { id: u.id } });
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  const users = data?.findUsers ?? [];

  return (
    <>
      <div className="d-flex justify-content-between align-items-center mb-3">
        <h4>Users</h4>
        <Button variant="primary" onClick={() => setShowCreate(true)}>
          Add User
        </Button>
      </div>

      {loading ? (
        <p>Loading…</p>
      ) : (
        <Table responsive striped>
          <thead>
            <tr>
              <th>Username</th>
              <th>Role</th>
              <th>Status</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {users.map((u) => (
              <tr key={u.id}>
                <td>{u.username}</td>
                <td>{roleLabel[u.role]}</td>
                <td>{u.disabled ? "Disabled" : "Active"}</td>
                <td className="text-right">
                  <Button
                    size="sm"
                    variant="secondary"
                    className="mr-2"
                    onClick={() => setModalUser(u)}
                  >
                    Edit
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    className="mr-2"
                    onClick={() => setCapsUser(u)}
                  >
                    Permissions
                  </Button>
                  <Button
                    size="sm"
                    variant="danger"
                    onClick={() => onDelete(u)}
                  >
                    Delete
                  </Button>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr>
                <td colSpan={4}>No users yet.</td>
              </tr>
            )}
          </tbody>
        </Table>
      )}

      {showCreate && (
        <UserModal
          onClose={() => setShowCreate(false)}
          onSaved={() => refetch()}
        />
      )}
      {modalUser && (
        <UserModal
          user={modalUser}
          onClose={() => setModalUser(null)}
          onSaved={() => refetch()}
        />
      )}
      {capsUser && (
        <UserCapabilitiesModal
          userId={capsUser.id}
          username={capsUser.username}
          onClose={() => setCapsUser(null)}
          onSaved={() => refetch()}
        />
      )}
    </>
  );
};
