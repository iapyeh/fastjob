<script>
import { setContext } from 'svelte';
import { writable } from 'svelte/store';
import Colortext from "./Colortext.svelte";
import Layout from "./layout.svelte";
export let name = '??'
let user ={
    username:name,
    age:30
}
setContext('context',{
    namespace:'playground',
    user:user
})
$: who = user.userename + 'UU'
//export let setUsername = function(username) {user.update(u => u.username = username)}
//setUsername('guest')
</script>

<Layout/>

<div class="green draw">
<Colortext bind:name={who} colors={['#ff00ff','#ffff00']}/>
</div>
