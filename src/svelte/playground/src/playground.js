import Tree from  "./tree.svelte"

const tree = new Tree({
	target: document.body.querySelector('#playground'),
	props: {
		name: 'playground'
	}
});

export default tree;
