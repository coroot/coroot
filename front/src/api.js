import axios from "axios";

export default class Api {
    axios = axios.create({
        baseURL: '/api/',
        timeout: 30000,
    });

    appId(id) {
        const parts = id.split(':');
        return {
            ns: parts[0] !== '_' ? parts[0] : '',
            kind: parts[1],
            name: parts[3] ? ':'+parts[3] : parts[2],
        }
    }

    get(url, cb) {
        this.axios.get(url).then((response) => {
            cb(response.data, '');
        }).catch((error) => {
            if (error.code === 'ECONNABORTED') {
                cb(null, 'Request timeout');
                return;
            }
            cb(null, error.response.data.trim() || 'Something went wrong, please try again later.');
        })
    }

    getOverview(cb) {
        this.get(`overview`, cb);
    }

    getApplication(id, cb) {
        this.get(`app/${id}`, cb);
    }
}
