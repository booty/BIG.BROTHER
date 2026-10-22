// The packaged application must put room connection events on stdout.
const assert = require('node:assert/strict');
const {spawn} = require('node:child_process');
const {once} = require('node:events');
const net = require('node:net');
const path = require('node:path');
const WebSocket = require('../contract/node_modules/ws');
const binary=process.argv[2];
const configDir=process.argv[3];
if(!binary || !configDir)throw new Error('Usage: node tests/server/logging.cjs BINARY CONFIG_DIR (set NODE_EXTRA_CA_CERTS to the test CA)');
async function until(predicate,description){
 const deadlineMs=Date.now()+5000;
 while(Date.now()<deadlineMs){if(predicate())return;await new Promise(resolve=>setTimeout(resolve,20));}
 throw new Error(`Timed out: ${description}`);
}
(async()=>{
 const listener=net.createServer();
 listener.listen(0,'127.0.0.1');await once(listener,'listening');
 const port=listener.address().port;await new Promise(resolve=>listener.close(resolve));
 const app=spawn(path.resolve(binary),['--config-dir',path.resolve(configDir),'--port',String(port)],{stdio:['ignore','pipe','pipe']});
 let stdout='',stderr='',exitCode;
 app.stdout.on('data',value=>{stdout+=value});app.stderr.on('data',value=>{stderr+=value});
 const exited=once(app,'exit').then(([code])=>{exitCode=code});
 let socket;
 try{
  await until(()=>stdout.includes('BIG.BROTHER listening at:') || exitCode!==undefined,'application startup');
  if(exitCode!==undefined)throw new Error(`Application exited: ${stderr}`);
  socket=new WebSocket(`wss://localhost:${port}/ws`);await once(socket,'open');
  socket.close();await once(socket,'close');
  await until(()=>stdout.includes('websocket disconnect') || stderr.includes('websocket disconnect'),'room disconnect log');
  for(const text of ['websocket connect','websocket disconnect','client_ip=','local_addr=','local_port='])assert.ok(stdout.includes(text),`stdout must contain ${text}; stderr=${stderr}`);
  assert.ok(!stderr.includes('websocket connect') && !stderr.includes('websocket disconnect'),'room connection events must not use stderr');
  console.log('PASS: packaged application logs room connection events and addresses to stdout');
 }finally{
  socket?.terminate();
  app.kill('SIGTERM');
  const timer=setTimeout(()=>app.kill('SIGKILL'),7000);
  await exited;clearTimeout(timer);
 }
})().catch(error=>{console.error(error);process.exitCode=1});
