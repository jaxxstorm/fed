import {execFileSync} from "node:child_process";
import {readFileSync,writeFileSync} from "node:fs";
import openapiTS,{astToString} from "openapi-typescript";
const schema=execFileSync("go",["run","./cmd/ui-contract"],{cwd:"..",encoding:"utf8"});
const types=astToString(await openapiTS(JSON.parse(schema)));
for(const [path,content] of [["openapi.json",schema],["src/generated.ts",types]]) {
 if(process.argv.includes("--check")) {if(readFileSync(path,"utf8")!==content)throw new Error(`${path} is outdated; run bun run contracts`)}
 else writeFileSync(path,content);
}
