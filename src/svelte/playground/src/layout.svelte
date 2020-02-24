<script>
import { onMount, getContext} from 'svelte';
import jQuery from "jquery";
import Headpane from './headpane.svelte'

const {namespace} = getContext('context')

onMount(async ()=>{
    const box = jQuery('#'+namespace+'Layout')
    box.css({
        width:'100%',
        height:'400px',
        'background-color':'#c0c0c0'
    })
    let layout = w2ui[namespace + 'layout']
    if (!layout){
        box.w2layout({
            name: namespace + 'layout',
            panels: [
                { type: 'top', size: 60 ,content:namespace},
                { type: 'left', size: 150, resizable: true },
                { type: 'right', size: 150, resizable: true}
            ]
        });
        layout = w2ui[namespace + 'layout']
        layout.hide('right')
        layout.headpane = new Headpane({
            target:layout.el('top')
        })
    }
})

</script>
<div class="layout" id="{namespace}Layout"></div>