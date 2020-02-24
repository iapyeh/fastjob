<script>
import {getContext} from 'svelte';
const {namespace,user} = getContext('context')
</script>
<style>
.headpane{
    float:right;
}
</style>
<div class="headpane">
Welcome {user.username}
</div>
