import { ref } from "vue";
import type { EnvironmentName } from "@/stores/app";

export type TopbarBreadcrumbItem = {
  key: string;
  label: string;
  onClick?: () => void;
};

export type TopbarActions = {
  environment: string;
  showSyncButton: boolean;
  canSyncExecute: boolean;
  onChangeEnvironment: (next: EnvironmentName) => void;
  onSyncToProduction: () => void;
};

const breadcrumb = ref<TopbarBreadcrumbItem[] | null>(null);
const actions = ref<TopbarActions | null>(null);
const connected = ref(false);

export function useTopbarBridge() {
  return {
    connected,
    breadcrumb,
    actions,
    setConnected(next: boolean) {
      connected.value = next;
    },
    setBreadcrumb(next: TopbarBreadcrumbItem[] | null) {
      breadcrumb.value = next;
    },
    setActions(next: TopbarActions | null) {
      actions.value = next;
    }
  };
}
