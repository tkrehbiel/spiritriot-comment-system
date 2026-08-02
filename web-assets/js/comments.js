const apiEndpoint = (window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1')
    ? 'http://localhost:8080/'
    : 'https://api.endgameviable.com/'; // TODO: configuration
const googleClientId = '391758766657-o6rmsb31431m5g07e3ma49pc2h522ceo.apps.googleusercontent.com';

const authorId = 'spiritriot-author';
const emailId = 'spiritriot-email';
const commentId = 'spiritriot-comment';
const resultId = 'spiritriot-result';
const buttonId = 'spiritriot-submit';
const containerId = 'spiritriot-form-container';
const displayCommentsId = 'spiritriot-live-comments';

const commentSubmitted = "Comment submitted successfully."
const commentRejected = "Comment rejected. Some possible reasons for this: You forgot your email, it looks like spam, you're trying to hack my site, or it's a bug.";

// Helper to retrieve brand SVGs
function getGoogleIconSvg(size = 14) {
    return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" style="display: inline-block; vertical-align: middle;">
        <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
        <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
        <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22c-.22-.66-.35-1.36-.35-2.09z"/>
        <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"/>
    </svg>`;
}

function getMastodonIconSvg(size = 14) {
    return `<svg viewBox="0 0 24 24" width="${size}" height="${size}" fill="currentColor" style="display: inline-block; vertical-align: middle;">
        <path d="M23.27 8.59c-.22-4.82-3.53-7.05-5.86-7.49C15 1 12 1 12 1s-3 0-5.41.1c-2.33.44-5.64 2.67-5.86 7.49-.18 3.79-.1 8.57.1 11.23.03.33.1.65.23.95.49 1.13 1.7 2.22 3.8 2.5a14.54 14.54 0 0 0 7.14 0c2.1-.28 3.31-1.37 3.8-2.5.13-.3.2-.62.23-.95.2-2.66.28-7.44.1-11.23zm-5.8 8.25h-2.52v-5.43c0-1.36-.57-2.05-1.72-2.05-1.27 0-1.92.83-1.92 2.47v2.9h-2.52v-2.9c0-1.64-.65-2.47-1.92-2.47-1.15 0-1.72.69-1.72 2.05v5.43H5.15V9.75c0-1.37.35-2.46 1.05-3.26.72-.81 1.66-1.22 2.82-1.22 1.35 0 2.38.52 3.08 1.57L12 8.16l.9-1.32c.7-1.05 1.73-1.57 3.08-1.57 1.16 0 2.1.41 2.82 1.22.7.8 1.05 1.89 1.05 3.26v7.09z"/>
    </svg>`;
}

function getLocal(name) {
    const x = localStorage.getItem(name);
    if (x) {
        const element = document.getElementById(name);
        if (element) element.value = x;
    }
    return x;
}

function setLocal(name) {
    const element = document.getElementById(name);
    if (element) {
        const x = element.value;
        localStorage.setItem(name, x);
        return x;
    }
    return '';
}

let googleToken = '';
let indieAuthToken = '';
let mastodonToken = '';
let verifiedIdentity = '';

function loadGoogleScript() {
    return new Promise((resolve) => {
        if (window.google && window.google.accounts) {
            resolve();
            return;
        }
        const script = document.createElement('script');
        script.src = 'https://accounts.google.com/gsi/client';
        script.async = true;
        script.defer = true;
        script.onload = () => resolve();
        document.head.appendChild(script);
    });
}

// Helper to decode base64 JWT payload
function decodeJwt(token) {
    try {
        const base64Url = token.split('.')[1];
        const base64 = base64Url.replace(/-/g, '+').replace(/_/g, '/');
        const jsonPayload = decodeURIComponent(atob(base64).split('').map(function(c) {
            return '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2);
        }).join(''));
        return JSON.parse(jsonPayload);
    } catch (e) {
        return null;
    }
}

// Display UI changes when a user logs in via Google, Mastodon, or IndieAuth
function showSignedInState(name, identityType, identityValue) {
    const googleBtn = document.getElementById('spiritriot-google-btn-container');
    const mastodonBtn = document.getElementById('spiritriot-mastodon-login-btn');
    const indieAuthBtn = document.getElementById('spiritriot-indieauth-login-btn');
    const authButtonsContainer = document.querySelector('.spiritriot-auth-buttons');
    const separators = document.querySelectorAll('.spiritriot-auth-separator');
    const statusPanel = document.getElementById('spiritriot-auth-status');
    const userNameSpan = document.getElementById('spiritriot-auth-user-name');
    const authorInput = document.getElementById(authorId);
    const emailInput = document.getElementById(emailId);

    if (googleBtn) googleBtn.style.display = 'none';
    if (mastodonBtn) mastodonBtn.style.display = 'none';
    if (indieAuthBtn) indieAuthBtn.style.display = 'none';
    if (authButtonsContainer) authButtonsContainer.style.display = 'none';
    separators.forEach(el => el.style.display = 'none');

    const formGrid = document.querySelector('.spiritriot-form-grid');
    if (formGrid) formGrid.style.display = 'none';

    if (statusPanel && userNameSpan) {
        statusPanel.style.display = 'flex';
        statusPanel.className = 'spiritriot-auth-status';
        if (identityType === 'google') {
            statusPanel.classList.add('spiritriot-theme-google');
            userNameSpan.innerHTML = `${getGoogleIconSvg(16)}Signed in as <b>${name}</b>`;
        } else if (identityType === 'indieauth') {
            statusPanel.classList.add('spiritriot-theme-indieauth');
            userNameSpan.innerHTML = `Signed in via IndieAuth as <a href="${identityValue}" target="_blank"><b>${name}</b></a>`;
        } else if (identityType === 'mastodon') {
            statusPanel.classList.add('spiritriot-theme-mastodon');
            userNameSpan.innerHTML = `${getMastodonIconSvg(16)}Signed in via Mastodon as <a href="${identityValue}" target="_blank"><b>${name}</b></a>`;
        }
    }

    if (authorInput) {
        authorInput.value = name;
        authorInput.disabled = true;
    }
    if (emailInput) {
        if (identityType === 'indieauth' || identityType === 'mastodon') {
            emailInput.value = '';
            emailInput.placeholder = identityType === 'indieauth' ? '(Verified IndieAuth)' : '(Verified Mastodon)';
        } else {
            emailInput.value = identityValue;
        }
        emailInput.disabled = true;
    }
}

function handleGoogleCredentialResponse(response) {
    googleToken = response.credential;
    const payload = decodeJwt(googleToken);
    if (payload) {
        sessionStorage.setItem('spiritriot-google-token', googleToken);
        showSignedInState(payload.name, 'google', payload.email);
    }
}

function handleSignOut() {
    googleToken = '';
    indieAuthToken = '';
    mastodonToken = '';
    verifiedIdentity = '';

    sessionStorage.removeItem('spiritriot-google-token');
    sessionStorage.removeItem('spiritriot-indieauth-token');
    sessionStorage.removeItem('spiritriot-indieauth-identity');
    sessionStorage.removeItem('spiritriot-mastodon-token');
    sessionStorage.removeItem('spiritriot-mastodon-identity');

    const authorInput = document.getElementById(authorId);
    const emailInput = document.getElementById(emailId);
    if (authorInput) {
        authorInput.value = '';
        authorInput.disabled = false;
    }
    if (emailInput) {
        emailInput.value = '';
        emailInput.placeholder = '';
        emailInput.disabled = false;
    }

    presetForm();

    const googleBtn = document.getElementById('spiritriot-google-btn-container');
    const mastodonBtn = document.getElementById('spiritriot-mastodon-login-btn');
    const indieAuthBtn = document.getElementById('spiritriot-indieauth-login-btn');
    const authButtonsContainer = document.querySelector('.spiritriot-auth-buttons');
    const separators = document.querySelectorAll('.spiritriot-auth-separator');
    const statusPanel = document.getElementById('spiritriot-auth-status');

    if (googleBtn) googleBtn.style.display = 'block';
    if (mastodonBtn) mastodonBtn.style.display = 'inline-flex';
    if (indieAuthBtn) indieAuthBtn.style.display = 'inline-flex';
    if (authButtonsContainer) authButtonsContainer.style.display = 'flex';
    separators.forEach(el => el.style.display = 'flex');
    if (statusPanel) {
        statusPanel.style.display = 'none';
        statusPanel.className = 'spiritriot-auth-status';
    }

    const formGrid = document.querySelector('.spiritriot-form-grid');
    if (formGrid) formGrid.style.display = 'grid';

    if (window.google) {
        google.accounts.id.disableAutoSelect();
    }
}

function renderGoogleButton() {
    const btnContainer = document.getElementById('spiritriot-google-btn-container');
    if (btnContainer && window.google) {
        google.accounts.id.renderButton(
            btnContainer,
            { theme: 'outline', size: 'large' }
        );
    }
}

// Reusable session validator helper
function getValidSessionToken(tokenKey, identityKey) {
    const token = sessionStorage.getItem(tokenKey);
    if (!token) return null;

    const payload = decodeJwt(token);
    if (payload && payload.exp * 1000 > Date.now()) {
        return token;
    }
    // Session expired or corrupted, clean storage keys
    sessionStorage.removeItem(tokenKey);
    if (identityKey) sessionStorage.removeItem(identityKey);
    return null;
}

// Restore user session if JWT is saved and unexpired
function checkExistingSessions() {
    const savedGoogleToken = getValidSessionToken('spiritriot-google-token');
    if (savedGoogleToken) {
        googleToken = savedGoogleToken;
        const payload = decodeJwt(googleToken);
        showSignedInState(payload.name, 'google', payload.email);
        return;
    }

    const savedMastodonToken = getValidSessionToken('spiritriot-mastodon-token', 'spiritriot-mastodon-identity');
    const savedMastodonIdentity = sessionStorage.getItem('spiritriot-mastodon-identity');
    if (savedMastodonToken && savedMastodonIdentity) {
        mastodonToken = savedMastodonToken;
        verifiedIdentity = savedMastodonIdentity;
        
        let name = verifiedIdentity;
        const parts = verifiedIdentity.split('/');
        if (parts.length > 3) {
            name = parts[3] + '@' + parts[2];
        }
        showSignedInState(name, 'mastodon', verifiedIdentity);
        return;
    }

    const savedIndieAuthToken = getValidSessionToken('spiritriot-indieauth-token', 'spiritriot-indieauth-identity');
    const savedIndieAuthIdentity = sessionStorage.getItem('spiritriot-indieauth-identity');
    if (savedIndieAuthToken && savedIndieAuthIdentity) {
        indieAuthToken = savedIndieAuthToken;
        verifiedIdentity = savedIndieAuthIdentity;
        
        let name = verifiedIdentity;
        const parts = verifiedIdentity.split('/');
        if (parts.length > 0) {
            name = parts[parts.length - 1];
        }
        showSignedInState(name, 'indieauth', verifiedIdentity);
    }
}

function handleIndieAuthLogin() {
    const width = 500;
    const height = 650;
    const left = (window.screen.width - width) / 2;
    const top = (window.screen.height - height) / 2;

    const popup = window.open(
        `${apiEndpoint}auth/indieauth/prompt`,
        'indieauth_login',
        `width=${width},height=${height},top=${top},left=${left}`
    );
    if (!popup) {
        alert('Popup blocked! Please allow popups for this site to log in.');
    }
}

function handlePostMessage(event) {
    if (event.data && event.data.type === 'spiritriot-auth-success') {
        const token = event.data.token;
        const identity = event.data.identity;
        const name = event.data.name;
        const provider = event.data.provider || (identity.startsWith('http') && !identity.includes('@') ? 'indieauth' : 'mastodon');

        if (provider === 'mastodon') {
            mastodonToken = token;
            verifiedIdentity = identity;
            sessionStorage.setItem('spiritriot-mastodon-token', token);
            sessionStorage.setItem('spiritriot-mastodon-identity', identity);
            showSignedInState(name, 'mastodon', identity);
        } else {
            indieAuthToken = token;
            verifiedIdentity = identity;
            sessionStorage.setItem('spiritriot-indieauth-token', token);
            sessionStorage.setItem('spiritriot-indieauth-identity', identity);
            showSignedInState(name, 'indieauth', identity);
        }
    }
}

function handleMastodonLogin() {
    const width = 500;
    const height = 650;
    const left = (window.screen.width - width) / 2;
    const top = (window.screen.height - height) / 2;

    const popup = window.open(
        `${apiEndpoint}auth/mastodon/prompt`,
        'mastodon_login',
        `width=${width},height=${height},top=${top},left=${left}`
    );
    if (!popup) {
        alert('Popup blocked! Please allow popups for this site to log in.');
    }
}

function addForm() {
    const container = document.getElementById(containerId);
    let html = `
<form id="spiritriot-form" class="spiritriot-form">
    <div class="spiritriot-auth-buttons">
        <div id="spiritriot-google-btn-container" class="spiritriot-google-btn-container"></div>
        <button type="button" id="spiritriot-mastodon-login-btn" class="spiritriot-button-mastodon">
            ${getMastodonIconSvg(16)}
            Sign in with Mastodon
        </button>
        <button type="button" id="spiritriot-indieauth-login-btn" class="spiritriot-button-indieauth">
            Sign in with IndieAuth
        </button>
    </div>
    
    <div class="spiritriot-auth-separator"><span>or</span></div>
    
    <div id="spiritriot-auth-status" class="spiritriot-auth-status" style="display: none;">
        <span id="spiritriot-auth-user-name" class="spiritriot-auth-user-name"></span>
        <button type="button" id="spiritriot-auth-signout" class="spiritriot-button-text">Sign Out</button>
    </div>

    <div class="spiritriot-form-grid">
        <div class="spiritriot-field">
            <label for="spiritriot-author">Name:</label>
            <input type="text" id="spiritriot-author" name="name" class="spiritriot-input" required>
        </div>
        <div class="spiritriot-field">
            <label for="spiritriot-email">Email:</label>
            <input type="text" id="spiritriot-email" name="email" class="spiritriot-input" required>
        </div>
    </div>

    <div class="spiritriot-field">
        <label for="spiritriot-comment">Comment (plain text please):</label>
        <textarea id="spiritriot-comment" name="comment" class="spiritriot-textarea" rows="4" required></textarea>
    </div>

    <div style="display:none;">
        <input type="text" id="website" name="website" value="">
    </div>

    <button id="spiritriot-submit" type="submit" class="spiritriot-button-primary">Submit</button>
</form>
<p id="spiritriot-result" class="spiritriot-result"></p>`
    container.innerHTML = html;
}

function presetForm() {
    getLocal(authorId);
    getLocal(emailId);
}

function disableSubmit() {
    const button = document.getElementById(buttonId);
    if (button) {
        button.disabled = true;
        button.innerText = 'Saving...';
    }
}

// Helper to enable form submit button
function enableSubmit() {
    const button = document.getElementById(buttonId);
    if (button) {
        button.disabled = false;
        button.innerText = 'Submit';
    }
}

// Helper to issue comment creation calls
function postComment(formData) {
    disableSubmit();
    return fetch(`${apiEndpoint}comment`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(formData)
    });
}

function handleSuccessfulSubmission(isPrivate) {
    document.getElementById(resultId).textContent = commentSubmitted;
    document.getElementById(commentId).value = '';
    if (!isPrivate) {
        fetchComments();
    }
}

function handleSubmit(event) {
    event.preventDefault();

    const username = document.getElementById(authorId).value;
    const email = document.getElementById(emailId).value;

    if (!googleToken && !indieAuthToken && !mastodonToken) {
        localStorage.setItem(authorId, username);
        localStorage.setItem(emailId, email);
    }

    const container = document.getElementById(containerId);
    const isPrivate = container && container.getAttribute('data-private') === 'true';

    const formData = {
        name: username,
        email: email,
        comment: document.getElementById(commentId).value,
        website: '',
        date: new Date().toISOString(),
        page: window.location.pathname,
        origin: window.location.href,
        private: isPrivate,
        google_token: googleToken,
        indieauth_token: indieAuthToken,
        mastodon_token: mastodonToken,
        migrate_account: false,
    };

    postComment(formData)
    .then(async response => {
        enableSubmit();
        if (response.status === 200) {
            handleSuccessfulSubmission(isPrivate);
        } else if (response.status === 409) {
            const text = await response.text();
            if (text === 'linking_consent_required') {
                const link = confirm(`This username is already registered to your email (${email}). Do you want to link your Google account to this username to verify future comments?`);
                if (link) {
                    resubmitWithMigration(formData);
                }
            } else {
                document.getElementById(resultId).textContent = text;
            }
        } else if (response.status === 403) {
            document.getElementById(resultId).textContent = commentRejected;
        } else {
            document.getElementById(resultId).textContent = 'Status ' + response.status;
        }
    })
    .catch(error => {
        enableSubmit();
        console.error('Error:', error);
        document.getElementById(resultId).textContent = 'Error submitting comment: ' + error;
    });
}

function resubmitWithMigration(formData) {
    formData.migrate_account = true;
    postComment(formData)
    .then(response => {
        enableSubmit();
        if (response.status === 200) {
            handleSuccessfulSubmission(formData.private);
        } else {
            document.getElementById(resultId).textContent = commentRejected;
        }
    })
    .catch(error => {
        enableSubmit();
        document.getElementById(resultId).textContent = 'Error linking account: ' + error;
    });
}

async function fetchComments() {
    const container = document.getElementById(displayCommentsId);
    if (!container) return;
    container.innerHTML = '<p>Checking for comments...</p>';

    fetch(`${apiEndpoint}comments?page=${window.location.pathname}`)
    .then(response => response.json())
    .then(comments => {
        if (comments.length > 0) {
            displayComments(comments);
        } else {
            container.innerHTML = '<p>No new comments found.</p>';
        }
    })
    .catch(error => {
        container.innerHTML = '<p>There was an error loading comments.</p>';
        console.error('Error:', error);
    });
}

function displayComments(comments) {
    const container = document.getElementById(displayCommentsId);
    if (!container) return;
    container.innerHTML = '';

    const headerElement = document.createElement('h3');
    headerElement.classList.add('spiritriot-comments-header');
    headerElement.textContent = `Recent Comments`;
    container.appendChild(headerElement);

    for (let i = 0; i < comments.length; i++) {
        const comment = comments[i];

        const commentCard = document.createElement('div');
        commentCard.classList.add('spiritriot-comment-card');
        if (comment.user_id) {
            commentCard.dataset.userId = comment.user_id;
        }

        const cardHeader = document.createElement('div');
        cardHeader.classList.add('spiritriot-comment-card-header');

        const authorMeta = document.createElement('div');
        authorMeta.classList.add('spiritriot-comment-author-meta');

        const authorElement = document.createElement('span');
        authorElement.classList.add('spiritriot-comment-author');

        if (comment.profile_url) {
            const linkElement = document.createElement('a');
            linkElement.href = comment.profile_url;
            linkElement.target = '_blank';
            linkElement.rel = 'nofollow noopener noreferrer';
            linkElement.textContent = comment.author;
            authorElement.appendChild(linkElement);
        } else {
            authorElement.textContent = comment.author;
        }
        authorMeta.appendChild(authorElement);

        if (comment.verified) {
            const isMastodon = comment.author && comment.author.startsWith('@');
            const isIndieAuth = comment.profile_url && !isMastodon;

            if (isMastodon) {
                const mastodonIcon = document.createElement('span');
                mastodonIcon.style.display = 'inline-flex';
                mastodonIcon.style.alignItems = 'center';
                mastodonIcon.style.color = '#563acc';
                mastodonIcon.style.marginLeft = '0.35rem';
                mastodonIcon.style.verticalAlign = 'middle';
                mastodonIcon.title = 'Verified Mastodon Account';
                mastodonIcon.innerHTML = getMastodonIconSvg(14);
                authorMeta.appendChild(mastodonIcon);
            } else if (isIndieAuth) {
                const verifiedElement = document.createElement('span');
                verifiedElement.classList.add('spiritriot-comment-verified');
                verifiedElement.textContent = '✔';
                verifiedElement.title = 'Verified IndieAuth Login';
                authorMeta.appendChild(verifiedElement);
            } else {
                // Google
                const googleIcon = document.createElement('span');
                googleIcon.style.display = 'inline-flex';
                googleIcon.style.alignItems = 'center';
                googleIcon.style.marginLeft = '0.35rem';
                googleIcon.style.verticalAlign = 'middle';
                googleIcon.title = 'Verified Google Account';
                googleIcon.innerHTML = getGoogleIconSvg(14);
                authorMeta.appendChild(googleIcon);
            }
        }
        cardHeader.appendChild(authorMeta);

        const dateElement = document.createElement('span');
        dateElement.classList.add('spiritriot-comment-date');
        dateElement.textContent = comment.date;
        cardHeader.appendChild(dateElement);

        commentCard.appendChild(cardHeader);

        const cardBody = document.createElement('div');
        cardBody.classList.add('spiritriot-comment-card-body');
        
        const contentPara = document.createElement('p');
        contentPara.textContent = comment.content;
        cardBody.appendChild(contentPara);

        commentCard.appendChild(cardBody);
        container.appendChild(commentCard);
    }
}

function startup() {
    addForm();
    document.getElementById('spiritriot-form').addEventListener('submit', handleSubmit);
    document.getElementById('spiritriot-auth-signout').addEventListener('click', handleSignOut);
    document.getElementById('spiritriot-indieauth-login-btn').addEventListener('click', handleIndieAuthLogin);
    document.getElementById('spiritriot-mastodon-login-btn').addEventListener('click', handleMastodonLogin);
    window.addEventListener('message', handlePostMessage);
    
    presetForm();
    checkExistingSessions();

    loadGoogleScript().then(() => {
        if (window.google) {
            google.accounts.id.initialize({
                client_id: googleClientId,
                callback: handleGoogleCredentialResponse,
                auto_select: false,
            });
            renderGoogleButton();
        }
    });

    const container = document.getElementById(containerId);
    const isPrivate = container && container.getAttribute('data-private') === 'true';
    if (!isPrivate) {
        fetchComments();
    }
}

document.addEventListener('DOMContentLoaded', startup);
