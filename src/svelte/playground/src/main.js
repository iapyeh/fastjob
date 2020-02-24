import App from './App.svelte';
import Tree from  "./tree.svelte"



const app = new App({
	target: document.body.querySelector('#app'),
	props: {
		name: 'world'
	}
});

app.Tree = Tree

export default app;