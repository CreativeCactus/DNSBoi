const http = require('http')
const dns = require('dns');
const express = require('express')

// Most apps should not have root, hence higher port numbers
const PORT = ":8000"

const app = express()
app.get('/health', (req,res)=>{
    console.log(`Someone asked how I am feeling!`)
    res.send("I am well :)")
})

const resolver = new dns.promises.Resolver();

main()
async function main(){
    /* Wait for service */
    await new Promise((a,r)=>{
        const I = setInterval(async ()=>{
            console.log("Waiting for DNSBoi...");            
            try {
                const res = await get('http://dnsboi:3353/health')
                if(res.statusCode!=200) throw res
                clearInterval(I)
                return a(res.data)
            } catch (e) {
                console.error(e)
            }
        },3000)
    })

    /* Set up DNS */
    await new Promise((a,r)=>{
        const I = setInterval(async ()=>{
            try {
                console.log(`Searching for CoreDNS IP...`)
                const coreDNSIP = `${await resolver.resolve4('coredns')}`
                console.log(`Found CoreDNS at ${coreDNSIP}...`)
                resolver.setServers([coreDNSIP]);
                clearInterval(I)
                return a(coreDNSIP)
            } catch (e) {
                console.error(e)
            }
        },3000)
    })

    /* Talk to friends */
    setInterval(async ()=>{
        console.log("Telling DNSBoi that I'm here...");
        try {
            const res = await get('http://dnsboi:3353/register?key=test&port=12345')
            console.log(res.data)
            if(res.statusCode != 200) throw res
            console.log("DNSBoi seems pleased, asking CoreDNS where I am...");
            console.log(await resolver.resolve('test.example.ntwrk'))
        } catch (e) {
            console.error(e)
        }
    }, 15000)

    app.listen(PORT, e=>console.log(`Listening ${PORT}: ${e||'All is well'}`))
}
async function get(url){
    return await new Promise((a,r)=>http.get(url,res=>{
        let data = '';
        res.on("data", (chunk) => data+=chunk);
        res.on("end", () => a({
            data,
            statusCode:res.statusCode
        }));
        res.on("error", r)
    }))
}