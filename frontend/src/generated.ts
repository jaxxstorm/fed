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
        AuthorizationHandoff: {
            /** Format: date-time */
            expires_at: string;
            operation_id: string;
            url: string;
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
        Form: {
            Action: string;
            Button: string;
            CSRF: string;
            Fields: components["schemas"]["Field"][] | null;
            Hints?: string[] | null;
            References?: components["schemas"]["ReferenceOption"][] | null;
            Title: string;
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
        Metric: {
            Label: string;
            Value: string;
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
        Page: {
            Activities: components["schemas"]["Link"][] | null;
            Citations: components["schemas"]["CitationCard"][] | null;
            Detail: boolean;
            Diagnostics: components["schemas"]["Section"][] | null;
            Error: string;
            Evidence: components["schemas"]["EvidenceCard"][] | null;
            Findings: components["schemas"]["Section"][] | null;
            Forms: components["schemas"]["Form"][] | null;
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
        RequirementNextStep: {
            action: string;
            explanation: string;
            provider?: string;
            targets: string[] | null;
            title: string;
        };
        RequirementPresentation: {
            action: string;
            evidence_complete: boolean;
            human_reviewed: boolean;
            label: string;
            next_steps?: components["schemas"]["RequirementNextStep"][] | null;
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
        SetupGuide: {
            callback_path?: string;
            configuration_names: string[] | null;
            execution: string;
            steps: string[] | null;
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
        UIBootstrap: {
            can_create: boolean;
            capabilities: {
                [key: string]: string[] | null;
            } | null;
            csrf: string;
            profiles?: {
                [key: string]: string;
            } | null;
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
    };
    responses: never;
    parameters: never;
    requestBodies: never;
    headers: never;
    pathItems: never;
}
export type $defs = Record<string, never>;
export type operations = Record<string, never>;
