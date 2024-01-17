export default {
	ts: {
		exec: "./myserver.ts",
		n: 4,
		proxy: {
			host: "localhost:3000",
			listen: ":3000",
			sticky: true,
			method: "conn",
			retry_max: 5,
			retry_backoff: 100,
		},
	},
};
