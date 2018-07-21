setInterval(()=>{
    console.log("Testing dnsboi...");
    try {
        require('http').get('http://dnsboi:3353/register')
    } catch (e) {
        console.error(e)
    }
}, 5000)

(async ()=>{
    await new Promise(()=>{})
})()