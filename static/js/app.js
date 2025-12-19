// static/js/app.js

const { createApp, ref, onMounted, nextTick } = Vue;
const { ElMessage } = ElementPlus;
const { Refresh } = ElementPlusIconsVue;

const app = createApp({
    setup() {
        const containers = ref([]);
        const loading = ref(false);
        const logVisible = ref(false);
        const logs = ref([]);
        const logBox = ref(null);
        let socket = null;

        // 获取列表
        const fetchContainers = () => {
            loading.value = true;
            fetch('/containers')
                .then(res => res.json())
                .then(data => {
                    containers.value = data;
                    loading.value = false;
                })
                .catch(err => {
                    ElMessage.error('获取列表失败');
                    loading.value = false;
                });
        };

        // 执行操作
        const handleAction = (id, action) => {
            loading.value = true;
            fetch(`/containers/${id}/${action}`, { method: 'POST' })
                .then(res => res.json())
                .then(data => {
                    if(data.error) {
                        ElMessage.error(data.error);
                    } else {
                        ElMessage.success(action === 'start' ? '启动指令已发送' : '停止指令已发送');
                        setTimeout(fetchContainers, 1000);
                    }
                })
                .catch(err => {
                    ElMessage.error('请求失败');
                    loading.value = false;
                });
        };

        // 打开控制台
        const openConsole = (id) => {
            logs.value = [];
            logVisible.value = true;
            const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = `${protocol}//${location.host}/containers/${id}/logs`;
            
            socket = new WebSocket(wsUrl);

            socket.onopen = () => logs.value.push({ text: ">>> 连接成功 <<<", style: "color: #2ecc71;" });
            
            socket.onmessage = (event) => {
                try {
                    const data = JSON.parse(event.data);
                    let colorStyle = "color: #bdc3c7;";
                    if (data.type === 'error') colorStyle = "color: #ff5555;";
                    else if (data.content.includes("INFO")) colorStyle = "color: #2ecc71;";
                    logs.value.push({ text: data.content, style: colorStyle });
                } catch (e) {
                    logs.value.push({ text: event.data, style: "color: #bdc3c7;" });
                }
                nextTick(() => {
                    if (logBox.value) logBox.value.scrollTop = logBox.value.scrollHeight;
                });
            };

            socket.onclose = () => logs.value.push({ text: ">>> 连接已断开 <<<", style: "color: #e67e22;" });
        };

        const closeConsole = (done) => {
            if (socket) socket.close();
            done();
        };

        onMounted(() => {
            fetchContainers();
        });

        return {
            containers, loading, logVisible, logs, logBox,
            fetchContainers, handleAction, openConsole, closeConsole, Refresh
        };
    }
});

app.use(ElementPlus);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    app.component(key, component)
}
app.mount('#app');