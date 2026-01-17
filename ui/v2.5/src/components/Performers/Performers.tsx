import React, { useMemo } from "react";
import { Route, Switch } from "react-router-dom";
import { Helmet } from "react-helmet";
import { useTitleProps } from "src/hooks/title";
import { useConfigurationContext } from "src/hooks/Config";
import { ListFilterModel } from "src/models/list-filter/filter";
import { applyHideMaleTransPerformers } from "src/core/performers";
import Performer from "./PerformerDetails/Performer";
import PerformerCreate from "./PerformerDetails/PerformerCreate";
import { PerformerList } from "./PerformerList";
import { View } from "../List/views";

const Performers: React.FC = () => {
  const { configuration } = useConfigurationContext();
  const showMaleTransPerformers =
    configuration?.ui?.showMaleTransPerformers ?? true;

  const filterHook = useMemo(() => {
    if (showMaleTransPerformers) return undefined;

    return (filter: ListFilterModel) => {
      return applyHideMaleTransPerformers(filter);
    };
  }, [showMaleTransPerformers]);

  return <PerformerList view={View.Performers} filterHook={filterHook} />;
};

const PerformerRoutes: React.FC = () => {
  const titleProps = useTitleProps({ id: "performers" });
  return (
    <>
      <Helmet {...titleProps} />
      <Switch>
        <Route exact path="/performers" component={Performers} />
        <Route path="/performers/new" component={PerformerCreate} />
        <Route path="/performers/:id/:tab?" component={Performer} />
      </Switch>
    </>
  );
};

export default PerformerRoutes;
