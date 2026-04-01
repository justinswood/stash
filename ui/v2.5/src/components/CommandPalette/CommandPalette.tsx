import React, { useEffect, useRef, useState, useMemo } from "react";
import { useHistory } from "react-router-dom";
import { useIntl, FormattedMessage } from "react-intl";
import { Icon } from "src/components/Shared/Icon";
import {
  faPlayCircle,
  faImage,
  faUser,
  faVideo,
  faTag,
  faImages,
  faFilm,
  faChartColumn,
  faCog,
  faClock,
  faSearch,
} from "@fortawesome/free-solid-svg-icons";
import "./CommandPalette.scss";

interface Command {
  id: string;
  label: string;
  icon: typeof faPlayCircle;
  action: () => void;
  keywords?: string;
}

interface IProps {
  show: boolean;
  onClose: () => void;
}

export const CommandPalette: React.FC<IProps> = ({ show, onClose }) => {
  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const history = useHistory();
  const intl = useIntl();

  const navigate = (path: string) => {
    history.push(path);
    onClose();
  };

  const commands: Command[] = useMemo(
    () => [
      {
        id: "scenes",
        label: intl.formatMessage({ id: "scenes" }),
        icon: faPlayCircle,
        action: () => navigate("/scenes"),
        keywords: "videos movies",
      },
      {
        id: "images",
        label: intl.formatMessage({ id: "images" }),
        icon: faImage,
        action: () => navigate("/images"),
        keywords: "photos pictures",
      },
      {
        id: "performers",
        label: intl.formatMessage({ id: "performers" }),
        icon: faUser,
        action: () => navigate("/performers"),
        keywords: "actors actresses people",
      },
      {
        id: "studios",
        label: intl.formatMessage({ id: "studios" }),
        icon: faVideo,
        action: () => navigate("/studios"),
        keywords: "companies producers",
      },
      {
        id: "tags",
        label: intl.formatMessage({ id: "tags" }),
        icon: faTag,
        action: () => navigate("/tags"),
        keywords: "categories labels",
      },
      {
        id: "galleries",
        label: intl.formatMessage({ id: "galleries" }),
        icon: faImages,
        action: () => navigate("/galleries"),
      },
      {
        id: "groups",
        label: intl.formatMessage({ id: "groups" }),
        icon: faFilm,
        action: () => navigate("/groups"),
        keywords: "movies series",
      },
      {
        id: "stats",
        label: intl.formatMessage({ id: "statistics" }),
        icon: faChartColumn,
        action: () => navigate("/stats"),
      },
      {
        id: "history",
        label: intl.formatMessage({
          id: "watch_history",
          defaultMessage: "Watch History",
        }),
        icon: faClock,
        action: () => navigate("/history"),
      },
      {
        id: "settings",
        label: intl.formatMessage({ id: "settings" }),
        icon: faCog,
        action: () => navigate("/settings"),
        keywords: "preferences configuration",
      },
      {
        id: "tagger",
        label: intl.formatMessage({ id: "sceneTagger" }),
        icon: faSearch,
        action: () => navigate("/scenes?tagger"),
        keywords: "match identify scrape",
      },
    ],
    [intl]
  );

  const filtered = useMemo(() => {
    if (!query) return commands;
    const q = query.toLowerCase();
    return commands.filter(
      (c) =>
        c.label.toLowerCase().includes(q) ||
        c.id.includes(q) ||
        c.keywords?.toLowerCase().includes(q)
    );
  }, [query, commands]);

  useEffect(() => {
    if (show) {
      setQuery("");
      setSelectedIndex(0);
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [show]);

  useEffect(() => {
    setSelectedIndex(0);
  }, [filtered.length]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setSelectedIndex((i) => Math.min(i + 1, filtered.length - 1));
        break;
      case "ArrowUp":
        e.preventDefault();
        setSelectedIndex((i) => Math.max(i - 1, 0));
        break;
      case "Enter":
        e.preventDefault();
        if (filtered[selectedIndex]) {
          filtered[selectedIndex].action();
        }
        break;
      case "Escape":
        onClose();
        break;
    }
  };

  if (!show) return null;

  return (
    <>
      <div className="command-palette-overlay" onClick={onClose} />
      <div className="command-palette" onKeyDown={handleKeyDown}>
        <div className="command-palette-input">
          <Icon icon={faSearch} className="command-palette-search-icon" />
          <input
            ref={inputRef}
            type="text"
            placeholder="Type a command..."
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            autoFocus
          />
        </div>
        <div className="command-palette-results">
          {filtered.map((cmd, idx) => (
            <button
              key={cmd.id}
              className={`command-palette-item${idx === selectedIndex ? " selected" : ""}`}
              onClick={cmd.action}
              onMouseEnter={() => setSelectedIndex(idx)}
            >
              <Icon icon={cmd.icon} className="command-palette-item-icon" />
              <span>{cmd.label}</span>
            </button>
          ))}
          {filtered.length === 0 && (
            <div className="command-palette-empty">No results</div>
          )}
        </div>
        <div className="command-palette-footer">
          <span>
            <kbd>↑↓</kbd> navigate
          </span>
          <span>
            <kbd>↵</kbd> select
          </span>
          <span>
            <kbd>esc</kbd> close
          </span>
        </div>
      </div>
    </>
  );
};
