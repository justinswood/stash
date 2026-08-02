import React, { useState } from "react";
import { Badge, Button, Form, Modal, Table } from "react-bootstrap";
import {
  GenderEnum,
  UserGroupDataFragment,
  useFindUserGroupsQuery,
  useFindUsersQuery,
  useUserGroupCreateMutation,
  useUserGroupUpdateMutation,
  useUserGroupDestroyMutation,
} from "src/core/generated-graphql";
import { TagIDSelect } from "src/components/Tags/TagSelect";
import { useToast } from "src/hooks/Toast";

const genderLabel: Record<GenderEnum, string> = {
  [GenderEnum.Male]: "Male",
  [GenderEnum.Female]: "Female",
  [GenderEnum.TransgenderMale]: "Transgender male",
  [GenderEnum.TransgenderFemale]: "Transgender female",
  [GenderEnum.Intersex]: "Intersex",
  [GenderEnum.NonBinary]: "Non-binary",
};

const allGenders = Object.values(GenderEnum);

interface IGroupModalProps {
  group?: UserGroupDataFragment; // undefined = create
  onClose: () => void;
  onSaved: () => void;
}

const GroupModal: React.FC<IGroupModalProps> = ({
  group,
  onClose,
  onSaved,
}) => {
  const Toast = useToast();
  const editing = !!group;

  const { data: usersData } = useFindUsersQuery();
  const [createGroup] = useUserGroupCreateMutation();
  const [updateGroup] = useUserGroupUpdateMutation();

  const [name, setName] = useState(group?.name ?? "");
  const [description, setDescription] = useState(group?.description ?? "");
  const [userIds, setUserIds] = useState<string[]>(
    group?.users.map((u) => u.id) ?? []
  );
  const [genders, setGenders] = useState<GenderEnum[]>(
    group?.excluded_genders ?? []
  );
  const [tagIds, setTagIds] = useState<string[]>(
    group?.excluded_tags.map((t) => t.id) ?? []
  );
  const [saving, setSaving] = useState(false);

  const users = usersData?.findUsers ?? [];

  function toggle<T>(list: T[], value: T): T[] {
    return list.includes(value)
      ? list.filter((v) => v !== value)
      : [...list, value];
  }

  async function onSave() {
    if (!name.trim()) return;
    setSaving(true);
    try {
      const fields = {
        name,
        description,
        user_ids: userIds,
        excluded_genders: genders,
        excluded_tag_ids: tagIds,
      };
      if (editing) {
        await updateGroup({ variables: { input: { id: group!.id, ...fields } } });
      } else {
        await createGroup({ variables: { input: fields } });
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
    <Modal show onHide={onClose} size="lg">
      <Modal.Header closeButton>
        <Modal.Title>{editing ? "Edit Group" : "Add Group"}</Modal.Title>
      </Modal.Header>
      <Modal.Body>
        <Form.Group className="mb-3">
          <Form.Label>Name</Form.Label>
          <Form.Control
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus={!editing}
          />
        </Form.Group>
        <Form.Group className="mb-3">
          <Form.Label>Description</Form.Label>
          <Form.Control
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </Form.Group>

        <h6 className="mt-4">Members</h6>
        <p className="text-muted">
          Accounts in this group. Restrictions from several groups combine, so
          membership only ever hides more.
        </p>
        {users.length === 0 ? (
          <p className="text-muted">No user accounts.</p>
        ) : (
          users.map((u) => (
            <Form.Check
              key={u.id}
              type="checkbox"
              id={`ug-user-${u.id}`}
              label={u.username}
              checked={userIds.includes(u.id)}
              onChange={() => setUserIds(toggle(userIds, u.id))}
            />
          ))
        )}

        <h6 className="mt-4">Hidden performer genders</h6>
        <p className="text-muted">
          Performers of these genders are hidden, along with any scene, image or
          gallery featuring one. Performers with no gender recorded stay visible.
        </p>
        {allGenders.map((g) => (
          <Form.Check
            key={g}
            type="checkbox"
            id={`ug-gender-${g}`}
            label={genderLabel[g]}
            checked={genders.includes(g)}
            onChange={() => setGenders(toggle(genders, g))}
          />
        ))}

        <h6 className="mt-4">Hidden tags</h6>
        <p className="text-muted">
          Content carrying these tags is hidden, as are performers carrying them.
          Pick tags explicitly — matching by name catches unrelated ones such as
          &quot;Transformation&quot; or &quot;Strap-on&quot;.
        </p>
        <TagIDSelect
          isMulti
          ids={tagIds}
          onSelect={(items) => setTagIds(items.map((i) => i.id))}
        />
      </Modal.Body>
      <Modal.Footer>
        <Button variant="secondary" onClick={onClose} disabled={saving}>
          Cancel
        </Button>
        <Button
          variant="primary"
          onClick={onSave}
          disabled={saving || !name.trim()}
        >
          Save
        </Button>
      </Modal.Footer>
    </Modal>
  );
};

export const SettingsUserGroupsPanel: React.FC = () => {
  const Toast = useToast();
  const { data, loading, refetch } = useFindUserGroupsQuery({
    fetchPolicy: "network-only",
  });
  const [destroyGroup] = useUserGroupDestroyMutation();
  const [modalGroup, setModalGroup] = useState<UserGroupDataFragment | null>(
    null
  );
  const [showCreate, setShowCreate] = useState(false);

  const groups = data?.findUserGroups ?? [];

  async function onDelete(g: UserGroupDataFragment) {
    if (
      !window.confirm(
        `Delete group "${g.name}"? Its members stop being restricted by it immediately.`
      )
    ) {
      return;
    }
    try {
      await destroyGroup({ variables: { id: g.id } });
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <>
      <div className="d-flex justify-content-between align-items-center mb-3">
        <h4>User Groups</h4>
        <Button variant="primary" onClick={() => setShowCreate(true)}>
          Add Group
        </Button>
      </div>
      <p className="text-muted">
        A group hides content from its members: performers of the chosen
        genders, anything featuring them, and anything carrying the chosen tags.
        Accounts in no group see everything. Library tasks such as scan and
        generate are never restricted.
      </p>

      {loading ? (
        <p>Loading…</p>
      ) : (
        <Table responsive striped>
          <thead>
            <tr>
              <th>Name</th>
              <th>Members</th>
              <th>Hidden genders</th>
              <th>Hidden tags</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {groups.map((g) => (
              <tr key={g.id}>
                <td>
                  {g.name}
                  {g.description && (
                    <div className="text-muted">{g.description}</div>
                  )}
                </td>
                <td>
                  {g.users.length === 0 ? (
                    <span className="text-muted">none</span>
                  ) : (
                    g.users.map((u) => (
                      <Badge key={u.id} variant="secondary" className="mr-1">
                        {u.username}
                      </Badge>
                    ))
                  )}
                </td>
                <td>
                  {g.excluded_genders.length === 0 ? (
                    <span className="text-muted">none</span>
                  ) : (
                    g.excluded_genders.map((x) => (
                      <Badge key={x} variant="warning" className="mr-1">
                        {genderLabel[x]}
                      </Badge>
                    ))
                  )}
                </td>
                <td>
                  {g.excluded_tags.length === 0 ? (
                    <span className="text-muted">none</span>
                  ) : (
                    <span title={g.excluded_tags.map((t) => t.name).join(", ")}>
                      {g.excluded_tags.length}
                    </span>
                  )}
                </td>
                <td className="text-right">
                  <Button
                    size="sm"
                    variant="secondary"
                    className="mr-2"
                    onClick={() => setModalGroup(g)}
                  >
                    Edit
                  </Button>
                  <Button size="sm" variant="danger" onClick={() => onDelete(g)}>
                    Delete
                  </Button>
                </td>
              </tr>
            ))}
            {groups.length === 0 && (
              <tr>
                <td colSpan={5}>
                  No groups — every account currently sees the whole library.
                </td>
              </tr>
            )}
          </tbody>
        </Table>
      )}

      {showCreate && (
        <GroupModal
          onClose={() => setShowCreate(false)}
          onSaved={() => refetch()}
        />
      )}
      {modalGroup && (
        <GroupModal
          group={modalGroup}
          onClose={() => setModalGroup(null)}
          onSaved={() => refetch()}
        />
      )}
    </>
  );
};
