// Exercise the real frontend with controlled media/room timing in headless Chromium.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const {execFileSync} = require('node:child_process');
const {pathToFileURL} = require('node:url');
const root=path.resolve(__dirname,'../..');
const esbuild=require(path.join(root,'web/node_modules/esbuild'));
const session=`bb-lifecycle-${process.pid}`;
const run=(...args)=>execFileSync('agent-browser',['--session',session,...args],{encoding:'utf8',timeout:20000});
function evaluate(code) {
 const value=JSON.parse(run('eval',`JSON.stringify((()=>{${code}})())`).trim());
 return typeof value==='string'?JSON.parse(value):value;
}
async function until(predicate,label) {
 const deadlineMs=Date.now()+4000;
 while(Date.now()<deadlineMs) {if(evaluate(predicate))return;await new Promise(resolve=>setTimeout(resolve,30));}
 throw new Error(`Timed out: ${label}; state=${JSON.stringify(evaluate('return window.__fixture.inspect()'))}`);
}
(async()=>{
 const directory=fs.mkdtempSync(path.join(root,'tmp/media-lifecycle-'));
 const devicesPath=path.join(root,'web/src/devices.ts');
 const mocks={
 './devices':`
 export {activeDeviceLabel,populateDeviceSelect} from ${JSON.stringify(devicesPath)};
 import {captureInitialMedia as realCapture} from ${JSON.stringify(devicesPath)};
 export async function enumerateDevices(){return {audioInputs:[],videoInputs:[]}}
 export function captureInitialMedia(...args){
  const f=window.__fixture;f.captureStarted=true;
  if(f.mode.startsWith('fallback'))return realCapture(...args);
  if(f.mode==='capture')return new Promise(resolve=>{f.resolveCapture=()=>{const stream=f.newStream();resolve({stream,errors:[]})}});
  return Promise.resolve({stream:f.newStream(),errors:[]});
 }
 export function captureDevice(){return new Promise(resolve=>{window.__fixture.resolveDevice=()=>resolve(window.__fixture.newStream().getVideoTracks()[0])})}
 `,
 './room':`
 export async function joinRoom(name,onMessage,onClose){
  const f=window.__fixture;const id='self'+(++f.joins);
  f.disconnect=onClose;f.onMessage=onMessage;
  f.self={id,name,audioEnabled:false,videoEnabled:false,streamId:id,publishUrl:'unused',readUrl:'unused',streamReady:false};
  onMessage({type:'welcome',payload:{selfId:id,participants:[f.self]}});
  return {selfId:id,send(){},close(){onClose()}};
 }
 `,
 './media':`
 export function publishMedia(){
  const f=window.__fixture;f.publishCount++;
  const session={stream:new MediaStream(),async replaceTrack(){},async close(){f.publicationCloses++}};
  if(f.mode==='failed-publication')return Promise.reject(new Error('fixture failure'));
  if(f.mode==='publication')return new Promise(resolve=>{f.resolvePublication=()=>resolve(session)});
  return Promise.resolve(session);
 }
 export function subscribeMedia(){
  const f=window.__fixture;return new Promise(resolve=>{f.resolveSubscription=()=>resolve({stream:new MediaStream(),async replaceTrack(){},async close(){f.subscriptionCloses++}})});
 }
 `};
 const main=fs.readFileSync(path.join(root,'web/src/main.ts'),'utf8')+`
 window.__fixture.inspect=()=>({closing,hasRoom:!!connection,hasPublication:!!publication,remoteCount:remoteSessions.size,
 localTrackIds:localStream?.getTracks().map(t=>t.id)??[],tracks:window.__fixture.tracks.map(t=>({id:t.id,state:t.readyState})),
 publishCount:window.__fixture.publishCount,publicationCloses:window.__fixture.publicationCloses,subscriptionCloses:window.__fixture.subscriptionCloses});`;
 await esbuild.build({stdin:{contents:main,resolveDir:path.join(root,'web/src'),loader:'ts'},bundle:true,format:'iife',outfile:path.join(directory,'app.js'),plugins:[{name:'timing-fixture',setup(build){
  build.onResolve({filter:/^\.\/(devices|room|media)$/},args=>args.importer==='<stdin>'?{path:args.path,namespace:'fixture'}:undefined);
  build.onLoad({filter:/.*/,namespace:'fixture'},args=>({contents:mocks[args.path],loader:'ts',resolveDir:root}));
 }}]});
 const bootstrap=`window.__fixture={mode:'auto',joins:0,tracks:[],publishCount:0,publicationCloses:0,subscriptionCloses:0,
 newStream(){const canvas=document.createElement('canvas');canvas.width=16;canvas.height=16;const stream=canvas.captureStream(30);this.tracks.push(...stream.getTracks());let frame=0;const timer=setInterval(()=>{if(stream.getTracks().every(t=>t.readyState==='ended')){clearInterval(timer);return}const ctx=canvas.getContext('2d');ctx.fillStyle=(frame++%2)?'red':'blue';ctx.fillRect(0,0,16,16)},30);return stream}};`;
 const html=fs.readFileSync(path.join(root,'web/index.html'),'utf8').replace('<script type="module" src="/app.js"></script>',`<script>${bootstrap}</script><script src="app.js"></script>`);
 fs.writeFileSync(path.join(directory,'index.html'),html);
 const open=mode=>{run('open',pathToFileURL(path.join(directory,'index.html')).href);evaluate(`window.__fixture.mode=${JSON.stringify(mode)};return true`);run('click','#join-button')};
 try {
  open('capture');
  await until('return !!window.__fixture.resolveCapture','capture pending');
  evaluate('window.__fixture.disconnect();return true');
  await until('return window.__fixture.inspect().closing','room cleanup');
  evaluate('window.__fixture.resolveCapture();return true');
  await until('return window.__fixture.tracks.length>0 && window.__fixture.tracks.every(t=>t.readyState==="ended")','late capture is stopped');
  let state=evaluate('return window.__fixture.inspect()');
  assert.equal(state.publishCount,0,'late capture must not publish');
  console.log('PASS: capture completing after disconnect is stopped without publishing');

  for(const mode of ['fallback-partial','fallback-reject']) {
   run('open',pathToFileURL(path.join(directory,'index.html')).href);
   evaluate(`const f=window.__fixture;f.mode=${JSON.stringify(mode)};f.captureCalls=0;
    navigator.mediaDevices.getUserMedia=()=>{f.captureCalls++;if(f.captureCalls===1)return new Promise((resolve,reject)=>{f.rejectCombined=()=>reject(new Error('combined unavailable'));if(f.mode==='fallback-partial')f.rejectCombined()});
     if(f.captureCalls===2)return Promise.resolve(f.newStream());return new Promise(resolve=>{f.resolveFallback=()=>resolve(f.newStream())})};return true`);
   run('click','#join-button');
   await until(mode==='fallback-partial'?'return !!window.__fixture.resolveFallback && window.__fixture.tracks.length===1':'return !!window.__fixture.rejectCombined','real capture pending');
   evaluate('window.__fixture.disconnect();return true');
   if(mode==='fallback-partial') {
    await until('return window.__fixture.tracks.every(t=>t.readyState==="ended")','partial capture stopped while camera pending');
    evaluate('window.__fixture.resolveFallback();return true');
    await until('return window.__fixture.tracks.length===2 && window.__fixture.tracks.every(t=>t.readyState==="ended")','late fallback stopped');
   } else {
    evaluate('window.__fixture.rejectCombined();return true');
    await until('return !document.querySelector("#join-button").disabled','capture rejected');
    assert.equal(evaluate('return window.__fixture.captureCalls'),1,'no fallback requests after teardown');
   }
   console.log(`PASS: ${mode} honors room teardown`);
  }

  open('failed-publication');
  await until('return window.__fixture.publishCount===1 && !document.querySelector("#join-button").disabled','failed initial publication');
  evaluate('window.__fixture.mode="publication";document.querySelector("#video-device").dispatchEvent(new Event("change"));return true');
  await until('return !!window.__fixture.resolveDevice','replacement capture pending');
  evaluate('window.__fixture.resolveDevice();return true');
  await until('return !!window.__fixture.resolvePublication','replacement publication pending');
  evaluate('window.__fixture.disconnect();window.__fixture.resolvePublication();return true');
  await until('return window.__fixture.tracks.length===2 && window.__fixture.tracks.every(t=>t.readyState==="ended") && window.__fixture.publicationCloses===1','replacement and rollback tracks released');
  console.log('PASS: canceled republishing stops retained rollback tracks');

  open('publication');
  await until('return !!window.__fixture.resolvePublication','publication pending');
  evaluate('window.__fixture.disconnect();return true');
  await until('return window.__fixture.tracks.every(t=>t.readyState==="ended")','capture stops before negotiation finishes');
  evaluate('window.__fixture.resolvePublication();return true');
  await until('return window.__fixture.publicationCloses===1','late publication closed');
  assert.equal(evaluate('return window.__fixture.inspect()').hasPublication,false);
  console.log('PASS: publication completing after disconnect is closed');

  open('auto');
  await until('return window.__fixture.inspect().hasPublication && !document.querySelector("#join-button").disabled','initial publication');
  evaluate('document.querySelector("#video-device").dispatchEvent(new Event("change"));return true');
  await until('return !!window.__fixture.resolveDevice','device switch pending');
  evaluate('window.__fixture.disconnect();return true');
  evaluate('window.__fixture.resolveDevice();return true');
  await until('return window.__fixture.tracks.length===2 && window.__fixture.tracks.every(t=>t.readyState==="ended")','late device stopped');
  assert.equal(evaluate('return window.__fixture.inspect()').publishCount,1);
  console.log('PASS: device capture completing after disconnect is stopped');

  open('auto');
  await until('return window.__fixture.inspect().hasPublication && !document.querySelector("#join-button").disabled','initial publication');
  evaluate('const f=window.__fixture;f.onMessage({type:"participants",payload:{participants:[f.self,{...f.self,id:"remote",streamId:"remote",streamReady:true}]}});return true');
  await until('return !!window.__fixture.resolveSubscription','subscription pending');
  evaluate('window.__fixture.disconnect();return true');
  run('click','#join-button');
  await until('return window.__fixture.joins===2 && window.__fixture.inspect().hasPublication','new room session');
  evaluate('window.__fixture.resolveSubscription();return true');
  await until('return window.__fixture.subscriptionCloses===1','old subscription closed after reconnect');
  state=evaluate('return window.__fixture.inspect()');
  assert.equal(state.remoteCount,0);
  assert.equal(state.hasPublication,true,'old work must not destroy the new publication');
  console.log('PASS: old subscription cannot attach after room reconnect');
  fs.writeFileSync(path.join(directory,'result.json'),JSON.stringify({status:'passed',scenarios:7},null,2));
  console.log(`Evidence: ${directory}`);
 } finally {run('close')}
})().catch(error=>{console.error(error);process.exitCode=1});
