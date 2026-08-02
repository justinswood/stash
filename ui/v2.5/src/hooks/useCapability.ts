import { Capability, useMeQuery } from "src/core/generated-graphql";

interface ICapabilityState {
  /** True when the current account holds the capability. */
  allowed: boolean;
  /** True until `me` has resolved. Render nothing rather than flashing a control. */
  loading: boolean;
}

/**
 * Reports whether the signed-in account holds a capability.
 *
 * This is for hiding controls that would fail, not for enforcement — the server
 * checks every operation independently (permissionMiddleware). A client that
 * lies to itself here gains nothing.
 */
export function useCapability(capability: Capability): ICapabilityState {
  const { data, loading } = useMeQuery();

  if (loading) {
    return { allowed: false, loading: true };
  }

  // No account at all means no credentials are configured and the instance is
  // open — the backend resolves that case to the admin preset, so match it.
  if (!data?.me) {
    return { allowed: true, loading: false };
  }

  return {
    allowed: data.me.capabilities.includes(capability),
    loading: false,
  };
}
