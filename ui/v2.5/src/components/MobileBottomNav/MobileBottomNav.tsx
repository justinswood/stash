import React, { useState } from "react";
import { NavLink, useLocation } from "react-router-dom";
import { FormattedMessage } from "react-intl";
import { Icon } from "src/components/Shared/Icon";
import {
  faPlayCircle,
  faImage,
  faUser,
  faSearch,
  faEllipsisH,
  faImages,
  faFilm,
  faVideo,
  faTag,
  faChartColumn,
  faCog,
  faTimes,
  faHouse,
} from "@fortawesome/free-solid-svg-icons";
import { useConfigurationContext } from "src/hooks/Config";
import "./MobileBottomNav.scss";

interface BottomTab {
  id: string;
  messageId: string;
  defaultMessage: string;
  href: string;
  icon: typeof faPlayCircle;
}

const primaryTabs: BottomTab[] = [
  {
    id: "home",
    messageId: "home",
    defaultMessage: "Home",
    href: "/",
    icon: faHouse,
  },
  {
    id: "scenes",
    messageId: "scenes",
    defaultMessage: "Scenes",
    href: "/scenes",
    icon: faPlayCircle,
  },
  {
    id: "images",
    messageId: "images",
    defaultMessage: "Images",
    href: "/images",
    icon: faImage,
  },
  {
    id: "performers",
    messageId: "performers",
    defaultMessage: "Performers",
    href: "/performers",
    icon: faUser,
  },
];

const moreTabs: BottomTab[] = [
  {
    id: "galleries",
    messageId: "galleries",
    defaultMessage: "Galleries",
    href: "/galleries",
    icon: faImages,
  },
  {
    id: "groups",
    messageId: "groups",
    defaultMessage: "Groups",
    href: "/groups",
    icon: faFilm,
  },
  {
    id: "studios",
    messageId: "studios",
    defaultMessage: "Studios",
    href: "/studios",
    icon: faVideo,
  },
  {
    id: "tags",
    messageId: "tags",
    defaultMessage: "Tags",
    href: "/tags",
    icon: faTag,
  },
  {
    id: "stats",
    messageId: "statistics",
    defaultMessage: "Statistics",
    href: "/stats",
    icon: faChartColumn,
  },
  {
    id: "settings",
    messageId: "settings",
    defaultMessage: "Settings",
    href: "/settings",
    icon: faCog,
  },
];

export const MobileBottomNav: React.FC = () => {
  const [showMore, setShowMore] = useState(false);
  const location = useLocation();
  const { configuration } = useConfigurationContext();
  const cfgMenuItems = configuration?.interface.menuItems;

  // Filter tabs based on user config; "home" is a navigation shortcut,
  // not a content menu item, so it's always shown.
  const filterTab = (tab: BottomTab) => {
    if (tab.id === "home") return true;
    if (!cfgMenuItems) return true;
    return cfgMenuItems.includes(tab.id);
  };

  const visiblePrimary = primaryTabs.filter(filterTab);
  const visibleMore = moreTabs.filter(filterTab);

  const isActive = (href: string) =>
    href === "/"
      ? location.pathname === "/"
      : location.pathname.startsWith(href);

  return (
    <>
      {showMore && (
        <div
          className="mobile-bottom-nav-overlay"
          onClick={() => setShowMore(false)}
        />
      )}
      <nav className="mobile-bottom-nav">
        {showMore && (
          <div className="mobile-bottom-nav-sheet">
            {visibleMore.map((tab) => (
              <NavLink
                key={tab.id}
                to={tab.href}
                className={`mobile-bottom-nav-sheet-item${isActive(tab.href) ? " active" : ""}`}
                onClick={() => setShowMore(false)}
              >
                <Icon icon={tab.icon} />
                <span>
                  <FormattedMessage
                    id={tab.messageId}
                    defaultMessage={tab.defaultMessage}
                  />
                </span>
              </NavLink>
            ))}
          </div>
        )}
        <div className="mobile-bottom-nav-tabs">
          {visiblePrimary.map((tab) => (
            <NavLink
              key={tab.id}
              to={tab.href}
              className={`mobile-bottom-nav-tab${isActive(tab.href) ? " active" : ""}`}
            >
              <Icon icon={tab.icon} />
              <span className="mobile-bottom-nav-label">
                <FormattedMessage
                  id={tab.messageId}
                  defaultMessage={tab.defaultMessage}
                />
              </span>
            </NavLink>
          ))}
          <NavLink
            to="/scenes?q="
            className={`mobile-bottom-nav-tab${location.pathname === "/search" ? " active" : ""}`}
            onClick={(e) => {
              e.preventDefault();
              // Focus the search input if on scene page, otherwise navigate
              const searchInput = document.querySelector<HTMLInputElement>(
                ".navbar-performer-search input, .search-input"
              );
              if (searchInput) {
                searchInput.focus();
              }
            }}
          >
            <Icon icon={faSearch} />
            <span className="mobile-bottom-nav-label">Search</span>
          </NavLink>
          <button
            className={`mobile-bottom-nav-tab mobile-bottom-nav-more${showMore ? " active" : ""}`}
            onClick={() => setShowMore(!showMore)}
          >
            <Icon icon={showMore ? faTimes : faEllipsisH} />
            <span className="mobile-bottom-nav-label">More</span>
          </button>
        </div>
      </nav>
    </>
  );
};
