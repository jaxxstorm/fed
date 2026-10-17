import { afterEach, expect, it, vi } from "vitest";
import { APIError, request } from "./api";

afterEach(() => vi.unstubAllGlobals());
it("preserves session expiry and recovery detail", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify({
          code: "session_expired",
          title: "Sign in",
          explanation: "Your session has expired",
        }),
        { status: 401 },
      ),
    ),
  );
  try {
    await request("/api/ui/v1/bootstrap");
    throw new Error("expected failure");
  } catch (e) {
    expect(e).toBeInstanceOf(APIError);
    expect(e).toMatchObject({
      status: 401,
      code: "session_expired",
      title: "Sign in",
    });
  }
});
it("does not treat an HTML error page as empty successful data", async () => {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValue(
        new Response("<html>gateway error</html>", { status: 502 }),
      ),
  );
  await expect(request("/api/ui/v1/bootstrap")).rejects.toMatchObject({
    code: "unavailable",
    status: 502,
  });
});
