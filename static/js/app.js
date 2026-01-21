// static/js/app.js

const { createApp, ref, onMounted, nextTick, reactive } = Vue; // 引入 reactive
const { ElMessage } = ElementPlus;
// 引入图标
const { Refresh, Plus, Delete } = ElementPlusIconsVue;

const app = createApp({
    setup() {
        const containers = ref([]);
        const loading = ref(false);
        
        // 日志相关
        const logVisible = ref(false);
        const logs = ref([]);
        const logBox = ref(null);
        let socket = null;

        // ✨ 创建相关
        const createVisible = ref(false);
        const creating = ref(false);
        // 表单数据
        const createForm = reactive({
            name: '',
            port: '25565',
            dataPath: '',
            image: '', // 新增镜像字段
            envList: [] // 用于前端渲染的动态数组
        });

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

        // 执行操作 (启动/停止)
        const handleAction = (id, action) => {
            loading.value = true;
            fetch(`/containers/${id}/${action}`, { method: 'POST' })
                .then(res => res.json())
                .then(data => {
                    if(data.error) {
                        ElMessage.error(data.error);
                    } else {
                        ElMessage.success(action === 'start' ? '指令已发送' : '指令已发送');
                        setTimeout(fetchContainers, 1000);
                    }
                })
                .catch(err => {
                    ElMessage.error('请求失败');
                    loading.value = false;
                });
        };

        // 打开创建弹窗（重置表单）
        const openCreateDialog = () => {
            createForm.name = '';
            createForm.port = '25565';
            createForm.dataPath = '';
            createForm.image = '';
            // 默认给几个常用变量，方便你开 Create 服
            createForm.envList = [
                { key: 'TYPE', value: 'FABRIC' },
                { key: 'VERSION', value: '1.20.1' },
                { key: 'MEMORY', value: '4G' }
            ];
            createVisible.value = true;
        };

        // 添加/删除环境变量行
        const addEnv = () => createForm.envList.push({ key: '', value: '' });
        const removeEnv = (index) => createForm.envList.splice(index, 1);

        // 提交创建
        const submitCreate = () => {
            if (!createForm.name || !createForm.port) {
                ElMessage.warning('请填写名称和端口');
                return;
            }

            creating.value = true;

            // 1. 把数组转换成后端要的 Map 对象
            const envMap = {};
            createForm.envList.forEach(item => {
                if (item.key && item.value) {
                    envMap[item.key] = item.value;
                }
            });

            // 2. 构造 Payload
            const payload = {
                name: createForm.name,
                port: createForm.port,
                dataPath: createForm.dataPath,
                image: createForm.image,
                env: envMap // 👈 发送转换后的对象
            };

            // 3. 发送请求
            fetch('/containers/create', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            })
            .then(res => res.json())
            .then(data => {
                creating.value = false;
                if (data.error) {
                    ElMessage.error(data.error);
                } else {
                    ElMessage.success('创建成功！容器正在启动...');
                    createVisible.value = false;
                    setTimeout(fetchContainers, 2000); // 稍等一下再刷新
                }
            })
            .catch(err => {
                creating.value = false;
                ElMessage.error('网络请求失败');
            });
        };

        // WebSocket 日志 (保持不变)
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
            createVisible, creating, createForm, // 新增
            fetchContainers, handleAction, openConsole, closeConsole, 
            openCreateDialog, addEnv, removeEnv, submitCreate, // 新增方法
            Refresh, Plus, Delete // 图标
        };
    }
});

app.use(ElementPlus);
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
    app.component(key, component)
}
app.mount('#app');