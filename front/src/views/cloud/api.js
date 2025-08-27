import axios from 'axios';

const defaultErrorMessage = 'Something went wrong, please try again later.';

class API {
    url = window.coroot.cloud_url;
    token = '';
    axios = axios.create({
        baseURL: this.url + '/api',
        timeout: 60000,
        timeoutErrorMessage: 'Request timed out.',
    });

    request(req, cb) {
        req.headers = { Authorization: this.token };
        this.axios(req)
            .then((response) => {
                try {
                    cb(response.data, '', response.status);
                } catch (e) {
                    console.error(e);
                }
            })
            .catch((error) => {
                const status = error.response && error.response.status;
                const err = error.response && error.response.data && error.response.data.trim();
                cb(null, err || error.message || defaultErrorMessage, status);
            });
    }
    get(url, params, cb) {
        this.request({ method: 'get', url, params }, cb);
    }
    post(url, data, cb) {
        this.request({ method: 'post', url, data }, cb);
    }
}

export default new API();
