export interface paths {
    "/api/ui/v1/bootstrap": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIBootstrap"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/members": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    offset?: number;
                };
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized membership response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["MemberPage"];
                    };
                };
            };
        };
        put?: never;
        post: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["MembershipChange"];
                };
            };
            responses: {
                /** @description Committed organization change */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content?: never;
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/membership-status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized membership response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ApprovalStatus"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/organization": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized membership response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Organization"];
                    };
                };
            };
        };
        put?: never;
        post: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OrganizationUpdate"];
                };
            };
            responses: {
                /** @description Committed organization change */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content?: never;
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path?: never;
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/x-www-form-urlencoded": {
                        [key: string]: string;
                    };
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Page"] | {
                            redirect: string;
                        };
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/actions/{action}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                    action: "evidence-source" | "collect" | "collection-config" | "scope" | "preview-profile" | "profile" | "measure" | "evidence" | "draft" | "review" | "gap" | "check" | "token" | "revoke-token" | "destination" | "prepare";
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/x-www-form-urlencoded": {
                        [key: string]: string;
                    };
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Page"] | {
                            redirect: string;
                        };
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"][] | null;
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/authorize": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["AuthorizationHandoff"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/configure": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["ConfigureInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/configure-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SourceConfigurationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/delete": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/disable-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SourceOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/disconnect": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/discover": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["DiscoverySession"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/discover-page": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["ServerDiscoveryInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["DiscoveryReceipt"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/operations/{operation}/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    operation: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Operation"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/providers": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ProviderDefinition"][] | null;
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/readiness/{provider}/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    provider: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["SetupReadiness"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/rebind-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SourceOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/refresh-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["RefreshSourceInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/renew": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Operation"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/resolve-commit": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["ResolveCommitInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ResolvedCommit"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/restore": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/restore-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SourceOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/retire-source": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SourceOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/scope": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Record"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/select": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SelectionInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["SelectionReceipt"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/authorize": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["AuthorizationHandoff"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/aws-material": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["SetupMaterial"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/aws-rotate": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/aws-verify": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/begin": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["BeginSetupInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/cancel": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/complete": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["SetupOperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/update": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["UpdateSetupInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/setups/{setup}/status": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    setup: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Setup"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/test": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["OperationInput"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Operation"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/{connection}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    connection: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Detail"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/connections/{connection}/inventory": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    connection: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["InventoryPage"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/observations/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    id: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ObservationDetail"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/overview": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIOverview"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/pages/{page}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    page: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Page"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/pages/{page}/{requirement}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: never;
                header?: never;
                path: {
                    service: string;
                    page: "requirement" | "run";
                    requirement: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["Page"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/preview/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query: {
                    revision: number;
                };
                header?: never;
                path: {
                    service: string;
                    id: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": {
                            content: string;
                        };
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/readiness/capabilities/{id}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    limit?: number;
                    offset?: number;
                };
                header?: never;
                path: {
                    service: string;
                    id: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessServicePlan"];
                    };
                };
                /** @description Authorized browser response */
                404: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/readiness/coverage": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    limit?: number;
                    offset?: number;
                };
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessQualityPage"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/readiness/plan": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    limit?: number;
                    offset?: number;
                };
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessServicePlan"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/readiness/requirements": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    limit?: number;
                    cursor?: string;
                    group?: "evidence" | "review" | "scoping" | "maintain" | "excluded";
                };
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessQueue"];
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/readiness/work": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    limit?: number;
                    offset?: number;
                };
                header?: never;
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessWorkPage"];
                    };
                };
            };
        };
        put?: never;
        post: {
            parameters: {
                query?: never;
                header: {
                    "X-CSRF-Token": string;
                };
                path: {
                    service: string;
                };
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/json": components["schemas"]["ReadinessWorkChange"];
                };
            };
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["ReadinessWorkRecord"];
                    };
                };
                /** @description Authorized browser response */
                400: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
                /** @description Authorized browser response */
                409: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/api/ui/v1/services/{service}/records/{kind}": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get: {
            parameters: {
                query?: {
                    offset?: string;
                    q?: string;
                    source?: string;
                    freshness?: string;
                    requirement?: string;
                    type?: string;
                };
                header?: never;
                path: {
                    service: string;
                    kind: "evidence" | "rule_result" | "evidence_source" | "snapshot" | "publication";
                };
                cookie?: never;
            };
            requestBody?: never;
            responses: {
                /** @description Authorized browser response */
                200: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": {
                            has_more: boolean;
                            items: components["schemas"]["Record"][];
                            offset: number;
                        };
                    };
                };
            };
        };
        put?: never;
        post?: never;
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/connections/upload/aws": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/x-www-form-urlencoded": {
                        csrf_token: string;
                        /** Format: password */
                        external_id: string;
                        state: string;
                    };
                };
            };
            responses: {
                /** @description Redirect to the source connection and resumable operation status */
                303: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content?: never;
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
    "/connections/upload/kubernetes": {
        parameters: {
            query?: never;
            header?: never;
            path?: never;
            cookie?: never;
        };
        get?: never;
        put?: never;
        post: {
            parameters: {
                query?: never;
                header?: never;
                path?: never;
                cookie?: never;
            };
            requestBody: {
                content: {
                    "application/x-www-form-urlencoded": {
                        csrf_token: string;
                        /** Format: password */
                        kubeconfig: string;
                        state: string;
                    };
                };
            };
            responses: {
                /** @description Redirect to the source connection and resumable operation status */
                303: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content?: never;
                };
                /** @description Authorized browser response */
                403: {
                    headers: {
                        [name: string]: unknown;
                    };
                    content: {
                        "application/json": components["schemas"]["UIError"];
                    };
                };
            };
        };
        delete?: never;
        options?: never;
        head?: never;
        patch?: never;
        trace?: never;
    };
}
export type webhooks = Record<string, never>;
export interface components {
    schemas: {
        AccessHealth: {
            actor_id?: string;
            checked_at?: string | null;
            error_code?: string;
            generation: number;
            limitations: string[] | null;
            principal?: string;
            state: string;
            valid_until?: string | null;
            verified_account_id?: string;
        };
        ActionItem: {
            action: string;
            basis: string;
            owner: string;
            sources: string[] | null;
            title: string;
        };
        ApprovalStatus: {
            email: string;
            state: string;
        };
        Artifact: {
            configuration: {
                [key: string]: {
                    [key: string]: unknown;
                };
            } | null;
            schema: string;
        };
        AssertionEvidence: {
            acceptance?: components["schemas"]["ReadinessAcceptance"] | null;
            assertion_id: string;
            citations: components["schemas"]["Citation"][] | null;
            gaps?: components["schemas"]["AssertionGap"][] | null;
            obligation: string;
            rationale: string;
            result?: components["schemas"]["Ref"] | null;
            states: string[] | null;
        };
        AssertionGap: {
            kind: string;
            next_action: string;
            period?: string;
            reason: string;
            scope: string;
        };
        AuthorizationHandoff: {
            /** Format: date-time */
            expires_at: string;
            operation_id: string;
            url: string;
        };
        BeginSetupInput: {
            connection_id?: string;
            expected_revision?: number;
            generation?: number;
            idempotency_key: string;
            input: components["schemas"]["SetupInput"];
            provider: string;
            service_id: string;
        };
        Citation: {
            end_line?: number;
            evidence: components["schemas"]["Ref"];
            json_pointer?: string | null;
            start_line?: number;
        };
        CitationCard: {
            EndLine: number;
            Evidence: components["schemas"]["Record"];
            JSONPointer: string | null;
            Label: string;
            StartLine: number;
        };
        Configuration: {
            account_id: string;
            cluster_id?: string;
            context?: string;
            installation_id?: string;
            known_namespaces?: string[] | null;
            profile?: string;
            region?: string;
            role_arn?: string;
            server?: string;
        };
        ConfigureInput: {
            access_mode: string;
            configuration: components["schemas"]["Configuration"];
            expected_revision: number;
            id: string;
            idempotency_key: string;
            name: string;
            provider: string;
            service_id: string;
        };
        Connection: {
            access_mode: string;
            access_strategy?: string;
            actor_id: string;
            configuration: components["schemas"]["Configuration"];
            generation: number;
            id: string;
            imported: boolean;
            lifecycle: string;
            name: string;
            provider: string;
            revision: number;
            service_id: string;
            /** Format: date-time */
            updated_at: string;
            verified_account_id?: string;
        };
        Coverage: {
            complete: boolean;
            error?: string;
            expected: number;
            observed: number;
            recovery?: components["schemas"]["Recovery"] | null;
            state: string;
        };
        Detail: {
            access: components["schemas"]["AccessHealth"];
            connection: components["schemas"]["Connection"];
            discovered_count: number;
            remote_revocation?: string;
            selected_count: number;
        };
        DiscoveredResource: {
            available: boolean;
            default_branch?: string;
            /** Format: date-time */
            last_seen_at: string;
            locator: string;
            name: string;
            permissions?: {
                [key: string]: boolean;
            } | null;
            resource: components["schemas"]["ResourceKey"];
            source_id?: string;
        };
        DiscoveryReceipt: {
            accepted: number;
            access_mode: string;
            actor_id: string;
            coverage: components["schemas"]["KindCoverage"][] | null;
            limitations: string[] | null;
            method: string;
            next_cursor?: string;
            /** Format: date-time */
            observed_at: string;
            session_id: string;
            state: string;
        };
        DiscoverySession: {
            access_mode: string;
            actor_id: string;
            connection_id: string;
            coverage: components["schemas"]["KindCoverage"][] | null;
            finished_at?: string | null;
            generation: number;
            id: string;
            next_cursor?: string;
            service_id: string;
            /** Format: date-time */
            started_at: string;
            state: string;
        };
        Evaluation: {
            condition: string;
            explanation: string;
            outcome: string;
        };
        EvidenceCandidate: {
            description: string;
            evidence: components["schemas"]["Ref"];
            limitations: string[] | null;
            /** Format: date-time */
            observed_at: string;
            reason: string;
            state: string;
            target?: components["schemas"]["Target"] | null;
        };
        EvidenceCard: {
            Action: string;
            Metadata: string;
            Observed: string;
            Source: string;
            Target: string;
            Title: string;
            URL: string;
        };
        EvidenceSource: {
            access_hint?: string;
            account_id?: string;
            cluster?: string;
            cluster_id?: string;
            commit?: string;
            connection_id?: string;
            enabled: boolean;
            freshness_hours: number;
            lifecycle?: string;
            locator: string;
            name: string;
            namespace?: string;
            namespace_uid?: string;
            paths: string[] | null;
            provider_tenant?: string;
            region?: string;
            repository?: string;
            requirement_ids: string[] | null;
            resource_id?: string;
            resource_kind?: string;
            scope_revision: number;
            type: string;
        };
        Field: {
            Label: string;
            Name: string;
            Options: string[] | null;
            Type: string;
            Value: string;
        };
        Finding: {
            artifact_schema?: string;
            basis: string;
            condition: string;
            configuration?: {
                [key: string]: {
                    [key: string]: unknown;
                };
            } | null;
            coverage?: components["schemas"]["Coverage"] | null;
            explanation: string;
            freshness?: string;
            id: string;
            observation_id: string;
            /** Format: date-time */
            observed_at: string;
            outcome: string;
            target: components["schemas"]["Target"];
        };
        FindingExplanation: {
            expected: string;
            known: boolean;
            limitations: string;
            next_action: string;
            observed: string;
            owner_role: string;
            title: string;
            version: string;
        };
        FindingGroup: {
            affected_targets: number;
            condition: string;
            explanation: components["schemas"]["FindingExplanation"];
            findings: components["schemas"]["Finding"][] | null;
            freshness?: string;
            id: string;
            last_observed_at?: string | null;
            outcome: string;
        };
        Form: {
            Action: string;
            Button: string;
            CSRF: string;
            Fields: components["schemas"]["Field"][] | null;
            Hints?: string[] | null;
            References?: components["schemas"]["ReferenceOption"][] | null;
            Title: string;
        };
        InstructionBinding: {
            assertion_hash?: string;
            catalog_id: string;
            guidance_hash?: string;
            guidance_revision?: number;
            guidance_variant?: string;
            profile_revision: number;
            scope_revision: number;
            sources: components["schemas"]["Ref"][] | null;
            version: string;
        };
        InventoryPage: {
            generation: number;
            next_cursor?: string;
            resources: components["schemas"]["InventoryResource"][] | null;
        };
        InventoryResource: {
            access_mode: string;
            actor_id: string;
            available: boolean;
            binding_revision: number;
            bound_connection_id?: string;
            default_branch?: string;
            /** Format: date-time */
            last_seen_at: string;
            locator: string;
            name: string;
            permissions?: {
                [key: string]: boolean;
            } | null;
            resource: components["schemas"]["ResourceKey"];
            session_id: string;
            source_id?: string;
            source_lifecycle?: string;
            source_revision: number;
        };
        KindCoverage: {
            authoritative: boolean;
            complete: boolean;
            error_code?: string;
            kind: string;
            limitations: string[] | null;
        };
        Link: {
            Text: string;
            URL: string;
        };
        MemberPage: {
            members: components["schemas"]["Membership"][] | null;
            next_offset?: number | null;
        };
        Membership: {
            email: string;
            identity_id: string;
            name: string;
            organization_id: string;
            revision: number;
            role: string;
            state: string;
        };
        MembershipChange: {
            expected_revision: number;
            identity_id: string;
            operation: string;
            role: string;
        };
        Metric: {
            Label: string;
            Value: string;
        };
        ObservationDetail: {
            artifact_hash: string;
            id: string;
            observation: components["schemas"]["Submission"];
            /** Format: date-time */
            received_at: string;
            source_kind: string;
        };
        ObservationStatus: {
            checks?: components["schemas"]["Finding"][] | null;
            count: number;
            errors: string[] | null;
            expected: number;
            findings: components["schemas"]["Finding"][] | null;
            freshness: string;
            last_observed_at?: string | null;
            missing_measures: number;
            observed: number;
            recovery: components["schemas"]["Recovery"][] | null;
            sources: string[] | null;
            technical: string;
        };
        Operation: {
            access?: components["schemas"]["AccessHealth"] | null;
            connection_id: string;
            error_code?: string;
            id: string;
            state: string;
        };
        OperationInput: {
            connection_id: string;
            expected_revision: number;
            generation: number;
            idempotency_key: string;
            service_id: string;
        };
        Organization: {
            allowed_domains: string[] | null;
            id: string;
            name: string;
            revision: number;
        };
        OrganizationUpdate: {
            allowed_domains: string[] | null;
            expected_revision: number;
            name: string;
        };
        Page: {
            ActionExpectations: string[] | null;
            ActionItems: components["schemas"]["ActionItem"][] | null;
            Activities: components["schemas"]["Link"][] | null;
            Citations: components["schemas"]["CitationCard"][] | null;
            Detail: boolean;
            Diagnostics: components["schemas"]["Section"][] | null;
            Error: string;
            Evidence: components["schemas"]["EvidenceCard"][] | null;
            EvidenceCoverage?: components["schemas"]["RequirementEvidenceContext"] | null;
            Findings: components["schemas"]["Section"][] | null;
            Forms: components["schemas"]["Form"][] | null;
            Guidance?: components["schemas"]["ReadinessGuidanceContext"] | null;
            Links: components["schemas"]["Link"][] | null;
            Metrics: components["schemas"]["Metric"][] | null;
            Notice: string;
            Pagination: components["schemas"]["Link"][] | null;
            Primary: components["schemas"]["RequirementPresentation"] | null;
            Rows: components["schemas"]["Row"][] | null;
            Sections: components["schemas"]["Section"][] | null;
            Title: string;
        };
        ProgressCounts: {
            evidence_complete: number;
            not_applicable: number;
            optional: number;
            passing: number;
            recommended: number;
            required: number;
            reviews_remaining: number;
            scope_decisions: number;
            statuses: {
                [key: string]: number;
            } | null;
        };
        ProviderDefinition: {
            access_modes: string[] | null;
            evidence_operations: string[] | null;
            guide: components["schemas"]["SetupGuide"];
            id: string;
            name: string;
            permissions: string[] | null;
            resource_kinds: string[] | null;
            server_adapter_available: boolean;
        };
        ReadinessAcceptance: {
            alternatives: components["schemas"]["ReadinessEvidenceRoute"][] | null;
            fact: string;
            insufficient: string[] | null;
            provenance: components["schemas"]["ReadinessStatement"];
            scope: components["schemas"]["ReadinessStatement"];
            time_period: components["schemas"]["ReadinessStatement"];
        };
        ReadinessAlternative: {
            explanation: components["schemas"]["ReadinessStatement"];
            id: string;
            steps: components["schemas"]["ReadinessSteps"];
        };
        ReadinessAssertion: {
            acceptance: components["schemas"]["ReadinessAcceptance"];
            clauses: string[] | null;
            id: string;
            obligation: components["schemas"]["ReadinessStatement"];
        };
        ReadinessCandidateEvidence: {
            limitations: string[] | null;
            observed_at?: string | null;
            reason: string;
            reference: components["schemas"]["ReadinessEvidenceRef"];
            state: string;
        };
        ReadinessCapability: {
            alternatives: components["schemas"]["ReadinessAlternative"][] | null;
            dependencies: components["schemas"]["ReadinessDependency"][] | null;
            id: string;
            name: string;
            scope_kind: string;
            setup: components["schemas"]["ReadinessSteps"];
        };
        ReadinessCapabilityReadiness: {
            alternative_id?: string;
            alternatives: components["schemas"]["ReadinessAlternative"][] | null;
            blocked: boolean;
            capability_id: string;
            declaration?: components["schemas"]["ReadinessWorkRecord"] | null;
            evidence: components["schemas"]["ReadinessCandidateEvidence"][] | null;
            evidence_state: string;
            id: string;
            implementation: string;
            name: string;
            needs_revalidation: boolean;
            next_action: string;
            prerequisite_ids: string[] | null;
            rationale: string;
            requirement_ids: string[] | null;
            review: string;
            scope: components["schemas"]["ReadinessScopeKey"];
            steps: components["schemas"]["ReadinessSteps"];
            trigger: string;
        };
        ReadinessCitation: {
            anchor: string;
            content_hash: string;
            excerpt: string;
            excerpt_hash: string;
            id: string;
            path: string;
            source: string;
        };
        ReadinessCondition: {
            equals: string;
            fact: string;
            question: string;
        };
        ReadinessCoverageEntry: {
            applicability: string;
            document: string;
            playbook_id?: string;
            reason?: string;
            requirement_id: string;
            state: string;
            variant_id: string;
        };
        ReadinessDependency: {
            capability_id: string;
            condition?: components["schemas"]["ReadinessCondition"] | null;
            reason: components["schemas"]["ReadinessStatement"];
        };
        ReadinessDutyReadiness: {
            declaration?: components["schemas"]["ReadinessWorkRecord"] | null;
            duty: components["schemas"]["ReadinessDutyRef"];
            evidence: components["schemas"]["ReadinessCandidateEvidence"][] | null;
            evidence_state: string;
            id: string;
            implementation: string;
            needs_revalidation: boolean;
            next_action: string;
            requirement_id: string;
            review: string;
            step: components["schemas"]["ReadinessStep"];
            trigger: string;
        };
        ReadinessDutyRef: {
            playbook_id: string;
            step_id: string;
        };
        ReadinessEvidenceRef: {
            id: string;
            revision: number;
        };
        ReadinessEvidenceRoute: {
            artifacts: components["schemas"]["ReadinessStatement"][] | null;
            id: string;
            limitations: string;
            proves: string;
        };
        ReadinessGuidanceContext: {
            catalog_id: string;
            caveat?: string;
            guidance?: components["schemas"]["ReadinessPackage"] | null;
            label?: string;
            progress?: components["schemas"]["ReadinessRequirementProgress"] | null;
            reason?: string;
            release_hash?: string;
            release_revision?: number;
            state: string;
            variant_id: string;
        };
        ReadinessPackage: {
            capabilities: components["schemas"]["ReadinessCapability"][] | null;
            catalog_id: string;
            citations: components["schemas"]["ReadinessCitation"][] | null;
            format: string;
            playbooks: components["schemas"]["ReadinessPlaybook"][] | null;
            revision: number;
            sources: {
                [key: string]: components["schemas"]["ReadinessSource"];
            } | null;
        };
        ReadinessPlanQuestion: {
            capability_id?: string;
            declaration?: components["schemas"]["ReadinessWorkRecord"] | null;
            expected_value?: string;
            fact?: string;
            question: string;
            reason: string;
            requirement_id: string;
            scope?: components["schemas"]["ReadinessScopeKey"] | null;
        };
        ReadinessPlaybook: {
            applicability: components["schemas"]["ReadinessStatement"];
            assertions: components["schemas"]["ReadinessAssertion"][] | null;
            collect: components["schemas"]["ReadinessSteps"];
            decision_owner_role?: string;
            decision_question?: string;
            id: string;
            kind: string;
            no_prerequisites_reason?: string;
            official_statement: string;
            operate: components["schemas"]["ReadinessSteps"];
            prerequisites: components["schemas"]["ReadinessDependency"][] | null;
            purpose: components["schemas"]["ReadinessStatement"];
            requirement_id: string;
            review?: components["schemas"]["ReadinessReview"] | null;
            review_steps: components["schemas"]["ReadinessSteps"];
            revision: number;
            setup: components["schemas"]["ReadinessSteps"];
            variant_ids: string[] | null;
        };
        ReadinessQualityPage: {
            catalog_id: string;
            complete: boolean;
            count_unit: string;
            counts: {
                [key: string]: number;
            } | null;
            entries: components["schemas"]["ReadinessCoverageEntry"][] | null;
            next_offset?: number | null;
            offset: number;
            release_hash: string;
            release_revision: number;
            service_id: string;
            variant_count: number;
        };
        ReadinessQueue: {
            /** Format: date-time */
            as_of: string;
            binding: components["schemas"]["InstructionBinding"];
            count_unit: string;
            counts: {
                [key: string]: number;
            } | null;
            items: components["schemas"]["ReadinessRequirement"][] | null;
            next_cursor?: string;
            ordering: string;
            service_id: string;
            total: number;
            view_hash: string;
        };
        ReadinessRequirement: {
            applicability: string;
            group: string;
            name: string;
            presentation: components["schemas"]["RequirementPresentation"];
            requirement_id: string;
        };
        ReadinessRequirementProgress: {
            binding: components["schemas"]["ReadinessWorkBinding"];
            capabilities: components["schemas"]["ReadinessCapabilityReadiness"][] | null;
            duties: components["schemas"]["ReadinessDutyReadiness"][] | null;
            questions: components["schemas"]["ReadinessPlanQuestion"][] | null;
        };
        ReadinessReview: {
            comparison: components["schemas"]["ReadinessSourceComparison"];
            content_hash: string;
            /** Format: date-time */
            reviewed_at: string;
            reviewer_id: string;
            reviewer_type: string;
        };
        ReadinessReviewFinding: {
            citations: string[] | null;
            finding: string;
        };
        ReadinessScopeKey: {
            account_id?: string;
            cluster_id?: string;
            kind: string;
            namespace_uid?: string;
            region?: string;
            repository?: string;
        };
        ReadinessServicePlan: {
            binding: components["schemas"]["ReadinessWorkBinding"];
            capabilities: components["schemas"]["ReadinessCapabilityReadiness"][] | null;
            caveat?: string;
            count_unit: string;
            duties: components["schemas"]["ReadinessDutyReadiness"][] | null;
            duty_count: number;
            label?: string;
            next_offset?: number | null;
            offset: number;
            question_count: number;
            questions: components["schemas"]["ReadinessPlanQuestion"][] | null;
            service_id: string;
            sources?: {
                [key: string]: components["schemas"]["ReadinessSource"];
            } | null;
            state: string;
            total: number;
        };
        ReadinessSource: {
            commit: string;
            repository: string;
        };
        ReadinessSourceComparison: {
            applicability: components["schemas"]["ReadinessReviewFinding"];
            evidence: components["schemas"]["ReadinessReviewFinding"];
            limitations: string[] | null;
            obligations: components["schemas"]["ReadinessReviewFinding"];
            prerequisites: components["schemas"]["ReadinessReviewFinding"];
            timing: components["schemas"]["ReadinessReviewFinding"];
            uncertainties: components["schemas"]["ReadinessReviewFinding"][] | null;
        };
        ReadinessStatement: {
            basis: string;
            citations: string[] | null;
            rationale: string;
            text: string;
        };
        ReadinessStep: {
            action: components["schemas"]["ReadinessStatement"];
            completion_criteria: components["schemas"]["ReadinessStatement"][] | null;
            id: string;
            owner_role: string;
            timing?: components["schemas"]["ReadinessStatement"] | null;
            trigger: string;
        };
        ReadinessSteps: {
            items: components["schemas"]["ReadinessStep"][] | null;
            none_reason?: string;
        };
        ReadinessWorkBinding: {
            catalog_id: string;
            profile_revision: number;
            release_hash: string;
            release_revision: number;
            scope_revision: number;
        };
        ReadinessWorkChange: {
            binding: components["schemas"]["ReadinessWorkBinding"];
            data: components["schemas"]["ReadinessWorkData"];
            expected_revision: number;
            idempotency_key: string;
            service_id: string;
        };
        ReadinessWorkData: {
            alternative_id?: string;
            assignee_id?: string;
            capability_id: string;
            decisions: {
                [key: string]: string;
            } | null;
            duty?: components["schemas"]["ReadinessDutyRef"] | null;
            evidence: components["schemas"]["ReadinessEvidenceRef"][] | null;
            implementation: string;
            notes: string;
            owner_role: string;
            scope: components["schemas"]["ReadinessScopeKey"];
            trigger: string;
        };
        ReadinessWorkPage: {
            items: components["schemas"]["ReadinessWorkRecord"][] | null;
            next_offset?: number | null;
        };
        ReadinessWorkRecord: {
            actor_id: string;
            binding: components["schemas"]["ReadinessWorkBinding"];
            /** Format: date-time */
            created_at: string;
            data: components["schemas"]["ReadinessWorkData"];
            id: string;
            revision: number;
            service_id: string;
        };
        Record: {
            actor_id: string;
            /** Format: date-time */
            created_at: string;
            data: {
                [key: string]: unknown;
            };
            id: string;
            kind: string;
            requirement_id: string;
            revision: number;
            service_id: string;
        };
        Recovery: {
            action: string;
            code: string;
            connection_provider?: string;
            environment: string;
            explanation: string;
            retryable: boolean;
            target: string;
            title: string;
        };
        Ref: {
            id: string;
            revision: number;
        };
        ReferenceOption: {
            ID: string;
            Kind: string;
            Label: string;
            Revision: number;
        };
        RefreshSourceInput: {
            expected_revision: number;
            generation: number;
            idempotency_key: string;
            scope_revision: number;
            service_id: string;
            source_id: string;
        };
        RequirementEvidenceContext: {
            assertions: components["schemas"]["AssertionEvidence"][] | null;
            blockers: string[] | null;
            candidates: components["schemas"]["EvidenceCandidate"][] | null;
            check_groups: components["schemas"]["FindingGroup"][] | null;
            collection: components["schemas"]["ObservationStatus"];
            finding_groups: components["schemas"]["FindingGroup"][] | null;
            limitations: string[] | null;
        };
        RequirementNextStep: {
            action: string;
            explanation: string;
            provider?: string;
            targets: string[] | null;
            title: string;
        };
        RequirementPresentation: {
            action: string;
            context_gaps?: components["schemas"]["RequirementNextStep"][] | null;
            evidence_complete: boolean;
            finding_groups?: components["schemas"]["FindingGroup"][] | null;
            guidance_reason?: string;
            guidance_state?: string;
            human_reviewed: boolean;
            label: string;
            next_steps?: components["schemas"]["RequirementNextStep"][] | null;
            readiness?: components["schemas"]["ReadinessRequirementProgress"] | null;
            reason: string;
            review_remaining: boolean;
            secondary: string[] | null;
            status: string;
        };
        ResolveCommitInput: {
            connection_id: string;
            generation: number;
            ref: string;
            repository_id: string;
            service_id: string;
        };
        ResolvedCommit: {
            commit: string;
            generation: number;
            repository_id: string;
        };
        ResourceKey: {
            id: string;
            kind: string;
            provider: string;
            tenant: string;
        };
        Row: {
            Applicability: string;
            Freshness: string;
            ID: string;
            Name: string;
            Review: string;
            Technical: string;
            URL: string;
        };
        Section: {
            Text: string;
            Title: string;
        };
        SelectionInput: {
            add_to_scope: boolean;
            expected_scope_revision: number;
            idempotency_key: string;
            service_id: string;
            sources: components["schemas"]["SourceConfigurationInput"][] | null;
        };
        SelectionReceipt: {
            scope_additions: components["schemas"]["ResourceKey"][] | null;
            scope_revision: number;
            sources: components["schemas"]["Record"][] | null;
        };
        ServerDiscoveryInput: {
            cursor: string;
            idempotency_key: string;
            limit: number;
            service_id: string;
            session_id: string;
        };
        Service: {
            id: string;
            name: string;
            organization_id: string;
        };
        Setup: {
            actor_id: string;
            connection_id?: string;
            error_code?: string;
            /** Format: date-time */
            expires_at: string;
            id: string;
            input: components["schemas"]["SetupInput"];
            provider: string;
            revision: number;
            service_id: string;
            step: string;
            strategy: string;
            /** Format: date-time */
            updated_at: string;
            verified_account_id?: string;
        };
        SetupGuide: {
            callback_path?: string;
            configuration_names: string[] | null;
            execution: string;
            steps: string[] | null;
            version: string;
        };
        SetupInput: {
            account_id?: string;
            name: string;
            role_arn?: string;
        };
        SetupMaterial: {
            cli: string;
            instructions: string[] | null;
            read_policy: {
                [key: string]: unknown;
            };
            role_arn: string;
            trust_policy: {
                [key: string]: unknown;
            };
            version: string;
        };
        SetupOperationInput: {
            expected_revision: number;
            idempotency_key: string;
            service_id: string;
            setup_id: string;
        };
        SetupReadiness: {
            customer_actions: string[] | null;
            error_code?: string;
            missing_configuration: string[] | null;
            operator_actions: string[] | null;
            permissions: string[] | null;
            provider: string;
            ready: boolean;
            strategy: string;
            version: string;
        };
        SourceConfigurationInput: {
            connection_id: string;
            expected_binding_revision: number;
            expected_revision: number;
            generation: number;
            idempotency_key: string;
            resource: components["schemas"]["ResourceKey"];
            service_id: string;
            source: components["schemas"]["EvidenceSource"];
            source_id: string;
        };
        SourceOperationInput: {
            expected_binding_revision: number;
            expected_revision: number;
            idempotency_key: string;
            service_id: string;
            source_id: string;
            target_connection_id?: string;
            target_generation?: number;
        };
        Submission: {
            artifact: components["schemas"]["Artifact"];
            coverage: components["schemas"]["Coverage"];
            evaluations: components["schemas"]["Evaluation"][] | null;
            idempotency_key: string;
            limitations: string[] | null;
            links: components["schemas"]["Link"][] | null;
            method: string;
            /** Format: date-time */
            observed_at: string;
            profile_revision: number;
            scope_revision: number;
            service_id: string;
            target: components["schemas"]["Target"];
            tool_version: string;
        };
        Target: {
            account_id?: string;
            cluster?: string;
            cluster_id?: string;
            commit?: string;
            kind: string;
            namespace?: string;
            namespace_uid?: string;
            path?: string;
            region?: string;
            repository?: string;
            resource_id?: string;
            resource_kind?: string;
            uid?: string;
            worktree?: string;
        };
        UIBootstrap: {
            can_create: boolean;
            can_manage_organization?: boolean;
            capabilities: {
                [key: string]: string[] | null;
            } | null;
            csrf: string;
            guidance_enabled?: boolean;
            profiles?: {
                [key: string]: string;
            } | null;
            role?: string;
            services: components["schemas"]["Service"][] | null;
        };
        UIError: {
            action: string;
            code: string;
            explanation: string;
            field_errors?: {
                [key: string]: string;
            } | null;
            reference: string;
            retryable: boolean;
            title: string;
        };
        UIOverview: {
            /** Format: date-time */
            as_of: string;
            categories: {
                [key: string]: components["schemas"]["ProgressCounts"] | null;
            } | null;
            counts: components["schemas"]["ProgressCounts"];
            evidence_records: number;
            rules: components["schemas"]["UIRule"][] | null;
            setup_required: boolean;
        };
        UIRule: {
            applicability: string;
            category: string;
            id: string;
            kind: string;
            name: string;
            open_gap: boolean;
            state: components["schemas"]["RequirementPresentation"];
            statement: string;
        };
        UpdateSetupInput: {
            expected_revision: number;
            idempotency_key: string;
            input: components["schemas"]["SetupInput"];
            service_id: string;
            setup_id: string;
        };
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export type operations = Record<string, never>;
