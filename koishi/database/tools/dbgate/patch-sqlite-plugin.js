const fs = require('fs');
const path = process.argv[2];

let content = fs.readFileSync(path, 'utf8');

// Find the connect function for module 317 (sqlite@dbgate-plugin-sqlite)
// Look for connect:async({databaseFile:t,isReadOnly:n})=>({client:new(g||(g=e(550)),g)(t,{readonly:!!n})})
// We need to be careful with minification.

const oldStr = 'client:new(g||(g=e(550)),g)(t,{readonly:!!n})';
const newStr = 'client:(()=>{const db=new(g||(g=e(550)),g)(t,{readonly:!!n});try{db.loadExtension("/home/dbgate-docker/node_modules/sqlite-vec-linux-x64/vec0.so");}catch(e){console.error("Failed to load sqlite-vec:",e);}return db;})()';

if (content.includes(oldStr)) {
    content = content.replace(oldStr, newStr);
    fs.writeFileSync(path, content);
    console.log('Patched sqlite plugin successfully');
} else {
    console.error('Could not find connect function to patch');
    process.exit(1);
}
