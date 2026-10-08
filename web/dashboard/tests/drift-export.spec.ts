import { test, expect, type Page } from "@playwright/test";
import { createServer, type Server } from "node:http";
import { readFile } from "node:fs/promises";

const fixture = JSON.parse(await readFile(new URL("./fixture-state.json", import.meta.url), "utf8"));
fixture.result.analysis.graph.nodes = [{id:"node_codex_mcp",type:"MCP_SERVER",client:"codex",display_name:"Fixture MCP",adapter_confidence:"CONFIRMED"}];
fixture.result.analysis.findings[0].graph_references = ["node_codex_mcp"];
let server: Server, base: string, revision: number, names: string[], drift: unknown, calls: {path:string;body:any}[], failRefresh: boolean;
const changes = [
  {kind:"CHANGED",entity_type:"FINDING",id:"PATH-0001:0123456789abcdef",summary:"ignored",before:"RAW_BEFORE_CANARY",after:"RAW_AFTER_CANARY"},
  {kind:"REMOVED",entity_type:"NODE",id:"gone",summary:"ignored"},
  {kind:"ADDED",entity_type:"NODE",id:"node_codex_mcp",summary:"ignored"},
  {kind:"ADDED",entity_type:"CLIENT",id:"codex",summary:"ignored"},
];
test.beforeAll(async()=>{
 server=createServer(async(req,res)=>{
  const path=new URL(req.url!,"http://localhost").pathname;
  if(path.startsWith("/api/")) {
   let raw="";for await(const chunk of req) raw+=chunk;
   const body=raw?JSON.parse(raw):null;calls.push({path,body});res.setHeader("Content-Type","application/json");
   if(req.headers["x-harnessscope-token"]!=="drift-test") {res.writeHead(401).end(JSON.stringify({code:"unauthorized",message:"Session required",details:{}}));return;}
   if(body && body.revision!==revision) {res.writeHead(409).end(JSON.stringify({code:"stale_revision",message:"Snapshot changed",details:{}}));return;}
   if(path.endsWith("/state") && failRefresh) {res.writeHead(503).end(JSON.stringify({code:"unavailable",message:"Offline",details:{}}));return;}
   if(path.endsWith("/snapshots")) {if(body) {names.push(body.name);res.end(JSON.stringify({name:body.name,schema_version:"1.0.0",created_at:fixture.scanned_at}));}else res.end(JSON.stringify(names.map(name=>({name,schema_version:"1.0.0",created_at:fixture.scanned_at}))));return;}
   if(path.endsWith("/drift")) {revision++;drift={schema_version:"1.0.0",baseline:body.baseline,changes:body.baseline==="empty"?[]:changes};}
   if(path.endsWith("/rescan")) {revision++;drift=undefined;}
   if(path.endsWith("/export")) {res.setHeader("Content-Type","application/zip");res.end(Buffer.from([80,75,5,6,...Array(18).fill(0)]));return;}
   res.end(JSON.stringify({...fixture,revision,drift}));return;
  }
  const name=path==="/"?"index.html":path.slice("/assets/".length);
  if(!["index.html","dashboard.js","dashboard.css"].includes(name)){res.writeHead(404).end();return;}
  res.setHeader("Content-Type",name.endsWith(".js")?"text/javascript":name.endsWith(".css")?"text/css":"text/html");res.end(await readFile(new URL(`../../../internal/server/assets/${name}`,import.meta.url)));
 });
 await new Promise<void>(resolve=>server.listen(0,"127.0.0.1",resolve));const address=server.address();if(!address||typeof address==="string")throw Error("port");base=`http://127.0.0.1:${address.port}`;
});
test.beforeEach(()=>{revision=7;names=["base","empty"];drift=undefined;calls=[];failRefresh=false;});
test.afterAll(async()=>{await new Promise<void>(resolve=>server.close(()=>resolve()));});
const open=async(page:Page,view="drift")=>{await page.goto(`${base}/?view=${view}#token=drift-test`);await expect(page.getByRole("heading",{name:view==="drift"?"Saved baselines":"Diagnostic bundle"})).toBeVisible();};

test("baseline save and deterministic drift groups expose only normalized metadata",async({page})=>{
 await open(page);await expect(page.getByLabel("Baseline",{exact:true})).toContainText("base");
 await page.getByLabel("Snapshot name").fill("new-base");await page.getByRole("button",{name:"Save snapshot"}).click();
 await expect(page.getByLabel("Baseline",{exact:true})).toHaveValue("new-base");
 await page.getByLabel("Baseline",{exact:true}).selectOption("base");await page.getByRole("button",{name:"Compare baseline"}).click();
 await expect(page.getByText("REV 8",{exact:true})).toBeVisible();
 await expect(page.locator(".drift-group h3")).toHaveText(["Added · CLIENT","Added · NODE","Removed · NODE","Changed · FINDING"]);
 await expect(page.locator("body")).not.toContainText("RAW_");
 await page.getByLabel("Baseline",{exact:true}).selectOption("empty");await page.getByRole("button",{name:"Compare baseline"}).click();await expect(page.getByText("No normalized drift from this baseline.")).toBeVisible();
});
test("no baseline state is actionable",async({page})=>{names=[];await open(page);await expect(page.getByText("No saved baselines yet.")).toBeVisible();await expect(page.getByRole("button",{name:"Compare baseline"})).toBeDisabled();});
test("drift node and finding links open existing views and removed nodes stay explicit",async({page})=>{
 await open(page);await page.getByRole("button",{name:"Compare baseline"}).click();
 await expect(page.getByText("Not in current snapshot",{exact:true})).toBeVisible();
 await page.getByRole("button",{name:"Inspect node node_codex_mcp"}).click();await expect(page).toHaveURL(/view=graph/);await expect(page.locator('[data-node-id="node_codex_mcp"]')).toHaveAttribute("aria-pressed","true");
 await page.getByRole("link",{name:"Drift"}).click();await page.getByRole("button",{name:"View current findings for rule PATH-0001"}).click();
 await expect(page).toHaveURL(/view=findings/);await expect(page.getByRole("heading",{name:"Missing fixture command"})).toBeVisible();await expect(page.getByRole("heading",{name:"Client portability needs review"})).toHaveCount(0);
});
test("removed finding identity never links to a surviving same-rule finding",async({page})=>{
 // Baseline A references a removed node; current B references node_codex_mcp.
 // The server's fingerprints distinguish A and B even though their rule is shared.
 const removedID="PATH-0001:aaaaaaaaaaaaaaaa",currentID="PATH-0001:bbbbbbbbbbbbbbbb";
 drift={schema_version:"1.0.0",baseline:"base",changes:[
  {kind:"REMOVED",entity_type:"FINDING",id:removedID,summary:"ignored",before:"RAW_REMOVED_CANARY"},
  {kind:"CHANGED",entity_type:"FINDING",id:currentID,summary:"ignored",after:"RAW_CURRENT_CANARY"},
 ]};
 await open(page);
 const removed=page.locator(".drift-row").filter({has:page.getByText(removedID,{exact:true})});
 await expect(removed).toContainText("Not in current snapshot");await expect(removed.getByRole("button")).toHaveCount(0);
 const current=page.locator(".drift-row").filter({has:page.getByText(currentID,{exact:true})});
 await expect(current.getByRole("button",{name:"View current findings for rule PATH-0001"})).toBeVisible();
 await expect(page.locator("body")).not.toContainText("RAW_");
 await current.getByRole("button").click();await expect(page).toHaveURL(/view=findings/);
 await expect(page.getByRole("heading",{name:"Missing fixture command"})).toBeVisible();
});
test("stale save never overwrites a baseline after automatic refresh",async({page})=>{
 await open(page);await page.getByLabel("Snapshot name").fill("new-base");revision=10;await page.getByRole("button",{name:"Save snapshot"}).click();
 await expect(page.getByText("REV 10",{exact:true})).toBeVisible();await expect(page.getByRole("alert")).toContainText("retry");
 expect(calls.filter(c=>c.path.endsWith("/snapshots")&&c.body)).toEqual([{path:"/api/v1/snapshots",body:{revision:7,name:"new-base"}}]);expect(names).not.toContain("new-base");
});
for(const view of ["drift","export"]) test(`stale ${view} refreshes without replay and stays retryable`,async({page})=>{
 await open(page,view);revision=11;
 const button=page.getByRole("button",{name:view==="drift"?"Compare baseline":"Download diagnostic ZIP"});await expect(button).toBeEnabled();await button.click();
 await expect(page.getByText("REV 11",{exact:true})).toBeVisible();await expect(page.getByRole("alert")).toContainText("retry");
 expect(calls.filter(c=>c.path.endsWith(view==="drift"?"/drift":"/export"))).toHaveLength(1);
 revision=12;failRefresh=true;await button.click();await expect(page.getByRole("alert")).toContainText("refresh failed");await expect(button).toBeEnabled();
});
test("ZIP download shows privacy warning and revokes its typed Blob URL immediately",async({page})=>{
 await page.addInitScript(()=>{const events:string[]=[];(window as any).urlEvents=events;const create=URL.createObjectURL.bind(URL),revoke=URL.revokeObjectURL.bind(URL);URL.createObjectURL=(blob:Blob)=>{events.push(`create:${blob.type}`);return create(blob);};URL.revokeObjectURL=(url:string)=>{events.push("revoke");revoke(url);};});
 await open(page,"export");await expect(page.getByText(/Human review is required before public upload/)).toBeVisible();
 const download=page.waitForEvent("download");await page.getByRole("button",{name:"Download diagnostic ZIP"}).click();expect((await download).suggestedFilename()).toBe("harnessscope-diagnostic.zip");
 expect(await page.evaluate(()=>(window as any).urlEvents)).toEqual(["create:application/zip","revoke"]);
});
test("click failure still revokes the URL and re-enables download",async({page})=>{
 await page.addInitScript(()=>{(window as any).revoked=0;const revoke=URL.revokeObjectURL.bind(URL);URL.revokeObjectURL=(url:string)=>{(window as any).revoked++;revoke(url);};HTMLAnchorElement.prototype.click=()=>{throw new Error("Synthetic download failure");};});
 await open(page,"export");await page.getByRole("button",{name:"Download diagnostic ZIP"}).click();await expect(page.getByRole("alert")).toContainText("Synthetic download failure");expect(await page.evaluate(()=>(window as any).revoked)).toBe(1);await expect(page.getByRole("button",{name:"Download diagnostic ZIP"})).toBeEnabled();
});
test("export sends the published drift baseline and a 768px view remains readable",async({page},testInfo)=>{
 await page.setViewportSize({width:768,height:900});await open(page);await page.getByRole("button",{name:"Compare baseline"}).click();await expect(page.getByText("REV 8",{exact:true})).toBeVisible();
 await page.screenshot({path:testInfo.outputPath("drift-768.png"),fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
 await page.getByRole("link",{name:"Export"}).click();await expect(page.getByText("Includes normalized drift against base.")).toBeVisible();
 const download=page.waitForEvent("download");await page.getByRole("button",{name:"Download diagnostic ZIP"}).click();await download;
 expect(calls.filter(c=>c.path.endsWith("/export"))).toEqual([{path:"/api/v1/export",body:{revision:8,baseline:"base"}}]);await page.screenshot({path:testInfo.outputPath("export-768.png"),fullPage:true});expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
});
