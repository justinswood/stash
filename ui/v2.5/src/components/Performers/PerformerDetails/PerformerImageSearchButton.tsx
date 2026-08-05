import React from "react";
import { Dropdown, SplitButton } from "react-bootstrap";
import { useIntl } from "react-intl";

interface ISearchSite {
  id: string;
  label: string;
  url: (name: string) => string;
}

// The first entry is what a plain click on the button body opens; the rest are
// only reachable from the caret menu.
//
// DuckDuckGo leads because kp=-2 reliably disables SafeSearch, whereas Google
// increasingly filters or sign-in-gates adult image results and ignores
// safe=off.
//
// Babepedia links straight at the profile rather than a search page: /babe/ is
// the canonical profile URL its own scraper builds (Babepedia.py), and the
// profile *is* the gallery. The site sits behind Cloudflare, so its search-page
// URL format could not be verified from the server.
const searchSites: ISearchSite[] = [
  {
    id: "duckduckgo",
    label: "DuckDuckGo Images",
    url: (name) =>
      `https://duckduckgo.com/?q=${encodeURIComponent(
        `${name} porn pics`
      )}&iax=images&ia=images&kp=-2`,
  },
  {
    id: "google",
    label: "Google Images",
    url: (name) =>
      `https://www.google.com/search?q=${encodeURIComponent(
        `${name} porn pics`
      )}&udm=2`,
  },
  {
    id: "babepedia",
    label: "Babepedia",
    url: (name) =>
      `https://www.babepedia.com/babe/${encodeURIComponent(
        name.replace(/ /g, "_")
      )}`,
  },
  {
    id: "boobpedia",
    label: "Boobpedia",
    url: (name) =>
      `https://www.boobpedia.com/wiki/index.php?search=${encodeURIComponent(
        name
      )}`,
  },
];

// noreferrer is not just hygiene here: without it the search engine receives
// this Stash instance's hostname in the Referer header.
function openSearch(site: ISearchSite, name: string) {
  window.open(site.url(name), "_blank", "noopener,noreferrer");
}

interface IProps {
  name?: string;
}

export const PerformerImageSearchButton: React.FC<IProps> = ({ name }) => {
  const intl = useIntl();

  if (!name) return null;

  return (
    <SplitButton
      id="performer-image-search"
      // styles/_theme.scss hides every dropdown caret behind this opt-in class
      // (`:not(.show-carat) > .dropdown-toggle::after { content: none }`).
      // Without it the caret half renders as an empty stub.
      className="show-carat"
      variant="secondary"
      title={intl.formatMessage({ id: "actions.find_images" })}
      onClick={() => openSearch(searchSites[0], name)}
    >
      {searchSites.map((site) => (
        <Dropdown.Item key={site.id} onClick={() => openSearch(site, name)}>
          {site.label}
        </Dropdown.Item>
      ))}
    </SplitButton>
  );
};

export default PerformerImageSearchButton;
