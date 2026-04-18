import React, {
  useEffect,
  useRef,
  useState,
  useCallback,
  useMemo,
} from "react";
import {
  defineMessages,
  FormattedMessage,
  MessageDescriptor,
  useIntl,
} from "react-intl";
import { Nav, Navbar, Button, Dropdown } from "react-bootstrap";
import { IconDefinition } from "@fortawesome/fontawesome-svg-core";
import { LinkContainer } from "react-router-bootstrap";
import { Link, NavLink, useLocation, useHistory } from "react-router-dom";
import Mousetrap from "mousetrap";

import SessionUtils from "src/utils/session";
import { Icon } from "src/components/Shared/Icon";
import { useConfigurationContext } from "src/hooks/Config";
import { ManualStateContext } from "./Help/context";
import { SettingsButton } from "./SettingsButton";
import {
  faBars,
  faChartColumn,
  faFilm,
  faHardDrive,
  faImage,
  faImages,
  faClock,
  faSearch,
  faMapMarkerAlt,
  faPlayCircle,
  faQuestionCircle,
  faSignOutAlt,
  faTag,
  faTimes,
  faUser,
  faVideo,
} from "@fortawesome/free-solid-svg-icons";
import { baseURL } from "src/core/createClient";
import { PatchComponent } from "src/patch";
import { mutateMetadataScan } from "src/core/StashService";
import { useToast } from "src/hooks/Toast";
import { withoutTypename } from "src/utils/data";
import * as GQL from "src/core/generated-graphql";
import { queryFindPerformersForSelect } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { useDebounce } from "src/hooks/debounce";
import PerformerStashBoxModal, {
  IStashBox,
} from "./Performers/PerformerDetails/PerformerStashBoxModal";
import { stashboxDisplayName } from "src/utils/stashbox";

interface IMenuItem {
  name: string;
  message: MessageDescriptor;
  href: string;
  icon: IconDefinition;
  hotkey: string;
  userCreatable?: boolean;
}
const messages = defineMessages({
  scenes: {
    id: "scenes",
    defaultMessage: "Scenes",
  },
  images: {
    id: "images",
    defaultMessage: "Images",
  },
  groups: {
    id: "groups",
    defaultMessage: "Groups",
  },
  markers: {
    id: "markers",
    defaultMessage: "Markers",
  },
  performers: {
    id: "performers",
    defaultMessage: "Performers",
  },
  studios: {
    id: "studios",
    defaultMessage: "Studios",
  },
  tags: {
    id: "tags",
    defaultMessage: "Tags",
  },
  galleries: {
    id: "galleries",
    defaultMessage: "Galleries",
  },
  sceneTagger: {
    id: "sceneTagger",
    defaultMessage: "Scene Tagger",
  },
  recents: {
    id: "recents",
    defaultMessage: "Recents",
  },
  statistics: {
    id: "statistics",
    defaultMessage: "Statistics",
  },
});

const allMenuItems: IMenuItem[] = [
  {
    name: "scenes",
    message: messages.scenes,
    href: "/scenes",
    icon: faPlayCircle,
    hotkey: "g s",
  },
  {
    name: "images",
    message: messages.images,
    href: "/images",
    icon: faImage,
    hotkey: "g i",
  },
  {
    name: "groups",
    message: messages.groups,
    href: "/groups",
    icon: faFilm,
    hotkey: "g v",
    userCreatable: true,
  },
  {
    name: "markers",
    message: messages.markers,
    href: "/scenes/markers",
    icon: faMapMarkerAlt,
    hotkey: "g k",
  },
  {
    name: "galleries",
    message: messages.galleries,
    href: "/galleries",
    icon: faImages,
    hotkey: "g l",
    userCreatable: true,
  },
  {
    name: "performers",
    message: messages.performers,
    href: "/performers",
    icon: faUser,
    hotkey: "g p",
    userCreatable: true,
  },
  {
    name: "studios",
    message: messages.studios,
    href: "/studios",
    icon: faVideo,
    hotkey: "g u",
    userCreatable: true,
  },
  {
    name: "tags",
    message: messages.tags,
    href: "/tags",
    icon: faTag,
    hotkey: "g t",
    userCreatable: true,
  },
];

const newPathsList = allMenuItems
  .filter((item) => item.userCreatable)
  .map((item) => item.href);

const MainNavbarMenuItems = PatchComponent(
  "MainNavBar.MenuItems",
  (props: React.PropsWithChildren<{}>) => {
    return <Nav>{props.children}</Nav>;
  }
);

const MainNavbarUtilityItems = PatchComponent(
  "MainNavBar.UtilityItems",
  (props: React.PropsWithChildren<{}>) => {
    return <>{props.children}</>;
  }
);

export const MainNavbar: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const { configuration } = useConfigurationContext();
  const { openManual } = React.useContext(ManualStateContext);
  const Toast = useToast();

  const [expanded, setExpanded] = useState(false);

  // Show all menu items by default, unless config says otherwise
  const menuItems = useMemo(() => {
    let cfgMenuItems = configuration?.interface.menuItems;
    if (!cfgMenuItems) {
      return allMenuItems;
    }

    // translate old movies menu item to groups
    cfgMenuItems = cfgMenuItems.map((item) => {
      if (item === "movies") {
        return "groups";
      }
      return item;
    });

    return allMenuItems.filter((menuItem) =>
      cfgMenuItems!.includes(menuItem.name)
    );
  }, [configuration]);

  // react-bootstrap typing bug
  const navbarRef = useRef<HTMLElement | null>(null);
  const intl = useIntl();

  const maybeCollapse = useCallback(
    (event: Event) => {
      if (
        navbarRef.current &&
        event.target instanceof Node &&
        !navbarRef.current.contains(event.target)
      ) {
        setExpanded(false);
      }
    },
    [setExpanded]
  );

  useEffect(() => {
    if (expanded) {
      document.addEventListener("click", maybeCollapse);
      document.addEventListener("touchstart", maybeCollapse);
    }
    return () => {
      document.removeEventListener("click", maybeCollapse);
      document.removeEventListener("touchstart", maybeCollapse);
    };
  }, [expanded, maybeCollapse]);

  const goto = useCallback(
    (page: string) => {
      history.push(page);
      if (document.activeElement instanceof HTMLElement) {
        document.activeElement.blur();
      }
    },
    [history]
  );

  const pathname = location.pathname.replace(/\/$/, "");
  let newPath = newPathsList.includes(pathname) ? `${pathname}/new` : null;
  if (newPath !== null) {
    let queryParam = new URLSearchParams(location.search).get("q");
    if (queryParam) {
      newPath += "?q=" + encodeURIComponent(queryParam);
    }
  }

  // set up hotkeys
  useEffect(() => {
    Mousetrap.bind("?", () => openManual());
    Mousetrap.bind("g z", () => goto("/settings"));

    menuItems.forEach((item) =>
      Mousetrap.bind(item.hotkey, () => goto(item.href))
    );

    if (newPath) {
      Mousetrap.bind("n", () => history.push(String(newPath)));
    }

    return () => {
      Mousetrap.unbind("?");
      Mousetrap.unbind("g z");
      menuItems.forEach((item) => Mousetrap.unbind(item.hotkey));

      if (newPath) {
        Mousetrap.unbind("n");
      }
    };
  });

  function maybeRenderLogout() {
    if (SessionUtils.isLoggedIn()) {
      return (
        <Button
          className="minimal logout-button d-flex align-items-center"
          href={`${baseURL}logout`}
          title={intl.formatMessage({ id: "actions.logout" })}
        >
          <Icon icon={faSignOutAlt} />
        </Button>
      );
    }
  }

  const handleDismiss = useCallback(() => setExpanded(false), [setExpanded]);
  const [performerQuery, setPerformerQuery] = useState("");
  const [performerResults, setPerformerResults] = useState<
    GQL.FindPerformersForSelectQuery["findPerformers"]["performers"]
  >([]);
  const [performerLoading, setPerformerLoading] = useState(false);
  const [showPerformerResults, setShowPerformerResults] = useState(false);
  const [activePerformerIndex, setActivePerformerIndex] = useState(-1);
  const [showStashBoxModal, setShowStashBoxModal] = useState(false);
  const [stashBoxModalQuery, setStashBoxModalQuery] = useState("");

  function getDefaultScanOptions(): GQL.ScanMetadataInput {
    return {
      scanGenerateCovers: true,
      scanGeneratePreviews: false,
      scanGenerateImagePreviews: false,
      scanGenerateSprites: false,
      scanGeneratePhashes: false,
      scanGenerateThumbnails: false,
      scanGenerateClipPreviews: false,
    };
  }

  async function runScan(paths?: string[]) {
    try {
      const uiScanDefaults = configuration?.ui?.taskDefaults
        ?.scan as GQL.ScanMetadataInput | undefined;
      const scanDefaults = uiScanDefaults
        ? withoutTypename(uiScanDefaults)
        : configuration?.defaults?.scan
        ? withoutTypename(configuration.defaults.scan)
        : undefined;

      const scanInput = {
        ...(scanDefaults ?? getDefaultScanOptions()),
        ...(paths ? { paths } : {}),
      };

      await mutateMetadataScan(scanInput);
      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.scan" }) }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  const stashes = configuration?.general.stashes ?? [];

  const debouncedSearchPerformers = useDebounce(async (value: string) => {
    if (!value.trim()) {
      setPerformerResults([]);
      setActivePerformerIndex(-1);
      return;
    }

    const filter = new ListFilterModel(GQL.FilterMode.Performers);
    filter.searchTerm = value;
    filter.currentPage = 1;
    filter.itemsPerPage = 10;
    filter.sortBy = "name";
    filter.sortDirection = GQL.SortDirectionEnum.Asc;

    setPerformerLoading(true);
    try {
      const query = await queryFindPerformersForSelect(filter);
      setPerformerResults(query.data.findPerformers.performers.slice());
      setActivePerformerIndex(0);
    } catch {
      setPerformerResults([]);
      setActivePerformerIndex(-1);
    } finally {
      setPerformerLoading(false);
    }
  }, 250);

  useEffect(() => {
    debouncedSearchPerformers(performerQuery);
    return () => {
      debouncedSearchPerformers.cancel();
    };
  }, [debouncedSearchPerformers, performerQuery]);

  function onPerformerResultSelect(performerId: string) {
    setShowPerformerResults(false);
    setPerformerQuery("");
    history.push(`/performers/${performerId}`);
  }

  function onPerformerKeyDown(event: React.KeyboardEvent<HTMLInputElement>) {
    if (!showPerformerResults || performerResults.length === 0) {
      return;
    }

    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActivePerformerIndex((prev) =>
        Math.min(prev + 1, performerResults.length - 1)
      );
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActivePerformerIndex((prev) => Math.max(prev - 1, 0));
    } else if (event.key === "Enter" && activePerformerIndex >= 0) {
      event.preventDefault();
      const performer = performerResults[activePerformerIndex];
      if (performer) {
        onPerformerResultSelect(performer.id);
      }
    } else if (event.key === "Escape") {
      setShowPerformerResults(false);
    }
  }

  const stashBoxes = configuration?.general.stashBoxes ?? [];

  function onSearchStashDB() {
    setStashBoxModalQuery(performerQuery.trim());
    setShowPerformerResults(false);
    setShowStashBoxModal(true);
  }

  function onStashBoxPerformerSelected(performer: GQL.ScrapedPerformer) {
    setShowStashBoxModal(false);
    setPerformerQuery("");
    history.push("/performers/new", {
      scrapeResult: performer,
      stashBoxEndpoint: stashBoxes[0].endpoint,
    });
  }

  function renderUtilityButtons() {
    return (
      <>
        <div className="nav-utility navbar-performer-search">
          <Icon icon={faSearch} className="navbar-performer-search-icon" />
          <input
            className="form-control form-control-sm"
            placeholder=""
            aria-label={intl.formatMessage({ id: "actions.search" })}
            value={performerQuery}
            onFocus={() => setShowPerformerResults(true)}
            onBlur={() => {
              setTimeout(() => setShowPerformerResults(false), 150);
            }}
            onChange={(event) => setPerformerQuery(event.target.value)}
            onKeyDown={onPerformerKeyDown}
          />
          {showPerformerResults && performerQuery.trim() && (
            <div className="navbar-performer-search-results">
              {performerLoading && (
                <div className="navbar-performer-search-item">
                  <FormattedMessage
                    id="loading.generic"
                    defaultMessage="Loading..."
                  />
                </div>
              )}
              {!performerLoading && performerResults.length === 0 && (
                <>
                  <div className="navbar-performer-search-item">
                    <FormattedMessage
                      id="no_results_found"
                      defaultMessage="No performers found"
                    />
                  </div>
                  {stashBoxes.length > 0 && (
                    <button
                      className="navbar-performer-search-item navbar-stashdb-search"
                      type="button"
                      onMouseDown={onSearchStashDB}
                    >
                      <Icon icon={faSearch} className="mr-2" />
                      Search {stashboxDisplayName(stashBoxes[0].name, 0)} for
                      &ldquo;{performerQuery.trim()}&rdquo;
                    </button>
                  )}
                </>
              )}
              {!performerLoading &&
                performerResults.map((performer, index) => (
                  <button
                    key={performer.id}
                    className={
                      "navbar-performer-search-item" +
                      (index === activePerformerIndex ? " is-active" : "")
                    }
                    type="button"
                    onMouseDown={() => onPerformerResultSelect(performer.id)}
                  >
                    {performer.image_path ? (
                      <img
                        className="navbar-performer-search-thumb"
                        src={performer.image_path}
                        alt=""
                        loading="lazy"
                      />
                    ) : (
                      <span className="navbar-performer-search-thumb navbar-performer-search-thumb--placeholder">
                        <Icon icon={faUser} />
                      </span>
                    )}
                    <span className="navbar-performer-search-name">
                      {performer.name}
                    </span>
                  </button>
                ))}
            </div>
          )}
          {showStashBoxModal && stashBoxes.length > 0 && (
            <PerformerStashBoxModal
              instance={{ ...stashBoxes[0], index: 0 }}
              name={stashBoxModalQuery}
              onHide={() => setShowStashBoxModal(false)}
              onSelectPerformer={onStashBoxPerformerSelected}
            />
          )}
        </div>
        <NavLink
          className="nav-utility"
          exact
          to={{
            pathname: "/scenes",
            search: "?sortby=created_at&sortdir=desc&perPage=10&disp=3",
          }}
          onClick={handleDismiss}
        >
          <Button
            className="minimal d-flex align-items-center h-100"
            title={intl.formatMessage(messages.recents)}
          >
            <Icon icon={faClock} />
          </Button>
        </NavLink>
        <NavLink
          className="nav-utility"
          exact
          to="/stats"
          onClick={handleDismiss}
        >
          <Button
            className="minimal d-flex align-items-center h-100"
            title={intl.formatMessage({ id: "statistics" })}
          >
            <Icon icon={faChartColumn} />
          </Button>
        </NavLink>
        <Dropdown className="nav-utility nav-scan-dropdown" drop="down">
          <Dropdown.Toggle
            as={Button}
            className="minimal d-flex align-items-center h-100"
            title={intl.formatMessage({ id: "actions.scan" })}
          >
            <Icon icon={faHardDrive} />
          </Dropdown.Toggle>
          <Dropdown.Menu>
            <Dropdown.Item
              onClick={() => {
                handleDismiss();
                runScan();
              }}
            >
              <FormattedMessage id="actions.scan_all" defaultMessage="Scan All Libraries" />
            </Dropdown.Item>
            {stashes.length > 0 && <Dropdown.Divider />}
            {stashes.map((stash) => (
              <Dropdown.Item
                key={stash.path}
                onClick={() => {
                  handleDismiss();
                  runScan([stash.path]);
                }}
              >
                {stash.path}
              </Dropdown.Item>
            ))}
          </Dropdown.Menu>
        </Dropdown>
        <NavLink
          className="nav-utility"
          exact
          to="/settings"
          onClick={handleDismiss}
        >
          <SettingsButton />
        </NavLink>
        <Button
          className="nav-utility minimal"
          onClick={() => openManual()}
          title={intl.formatMessage({ id: "help" })}
        >
          <Icon icon={faQuestionCircle} />
        </Button>
        {maybeRenderLogout()}
      </>
    );
  }

  return (
    <>
      <Navbar
        collapseOnSelect
        fixed="top"
        variant="dark"
        bg="dark"
        className="top-nav"
        expand="xl"
        expanded={expanded}
        onToggle={setExpanded}
        ref={navbarRef}
      >
        <Navbar.Collapse className="bg-dark order-sm-1">
          <MainNavbarMenuItems>
            {menuItems.map(({ href, icon, message }) => (
              <Nav.Link
                eventKey={href}
                as="div"
                key={href}
                className="col-4 col-sm-3 col-md-2 col-lg-auto"
              >
                <LinkContainer activeClassName="active" exact to={href}>
                  <Button className="minimal p-4 p-xl-2 d-flex d-xl-inline-block flex-column justify-content-between align-items-center">
                    <Icon
                      {...{ icon }}
                      className="nav-menu-icon d-block d-xl-inline mb-2 mb-xl-0"
                    />
                    <span>{intl.formatMessage(message)}</span>
                  </Button>
                </LinkContainer>
              </Nav.Link>
            ))}
          </MainNavbarMenuItems>
          <Nav>
            <MainNavbarUtilityItems>
              {renderUtilityButtons()}
            </MainNavbarUtilityItems>
          </Nav>
        </Navbar.Collapse>

        <Navbar.Brand as="div" onClick={handleDismiss}>
          <Link to="/">
            <Button className="minimal brand-link d-inline-block">Stash</Button>
          </Link>
        </Navbar.Brand>

        <Nav className="navbar-buttons flex-row ml-auto order-xl-2">
          {!!newPath && (
            <div className="mr-2">
              <Link to={newPath}>
                <Button variant="primary">
                  <FormattedMessage id="new" defaultMessage="New" />
                </Button>
              </Link>
            </div>
          )}
          <MainNavbarUtilityItems>
            {renderUtilityButtons()}
          </MainNavbarUtilityItems>
          <Navbar.Toggle className="nav-menu-toggle ml-sm-2">
            <Icon icon={expanded ? faTimes : faBars} />
          </Navbar.Toggle>
        </Nav>
      </Navbar>
    </>
  );
};
