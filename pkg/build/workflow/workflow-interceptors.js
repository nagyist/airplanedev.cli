import { interceptors as apInterceptors } from "@airplane/workflow-runtime/internal";

// Need to export interceptors in this format so that Temporal can
// find them.
export const interceptors = apInterceptors;
