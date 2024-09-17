const apiEndpoint = 'https://api.endgameviable.com/'; // TODO: configuration
const confirmation = '123';

const authorId = 'spiritriot-author';
const emailId = 'spiritriot-email';
const commentId = 'spiritriot-comment';
const resultId = 'spiritriot-result';
const buttonId = 'spiritriot-submit';
const containerId = 'spiritriot-form-container';
const displayCommentsId = 'spiritriot-live-comments';

const commentSubmitted = "Comment submitted successfully."
const commentRejected = "Comment rejected. Some possible reasons for this: You forgot your email, it looks like spam, you're trying to hack my site, or it's a bug.";

function getLocal(name) {
    const x = localStorage.getItem(name);
    if (x) {
        document.getElementById(name).value = x;
    }
    return x;
}

function setLocal(name) {
    const x = document.getElementById(name).value;
    localStorage.setItem(name, x);
    return x;
}

function addForm() {
    const container = document.getElementById(containerId);
    let html = `
<form id="spiritriot-form">
    <label for="name">Name:</label>
    <input type="text" id="spiritriot-author" name="name" required>

    <label for="email">Email:</label>
    <input type="text" id="spiritriot-email" name="email" required>

    <label for="comment">Comment (plain text please):</label>
    <textarea id="spiritriot-comment" name="comment" rows="4" required></textarea>

    <div style="display:none;">
        <input type="text" id="website" name="website" value="">
        <input type="text" id="page" name="page" value="{{ .RelPermalink }}">
        <input type="text" id="origin" name="origin" value="{{ .Permalink }}">
    </div>

    <input id="spiritriot-submit" type="submit" value="Submit">
</form>
<p id="spiritriot-result"></p>`
    container.innerHTML = html;
}

function presetForm() {
    getLocal(authorId);
    getLocal(emailId);
}

function disableSubmit() {
    const button = document.getElementById(buttonId);
    button.disabled = true;
    button.textContent = 'Saving'
}

function enableSubmit() {
    const button = document.getElementById(buttonId);
    button.disabled = false;
    button.textContent = 'Submit'
}

function handleSubmit(event) {
    event.preventDefault();

    const username = setLocal(authorId);
    const email = setLocal(emailId);

    // Gather the form data
    const formData = {
        name: username,
        email: email,
        comment: document.getElementById(commentId).value,
        website: '',
        date: new Date().toISOString(),
        page: window.location.pathname,
        origin: window.location.href,
    };

    // Send the AJAX request using the Fetch API
    disableSubmit();
    fetch(`${apiEndpoint}comment`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(formData)
    })
    .then(response => {
        enableSubmit();
        if (response.status === 200) {
            console.log('200 OK');
            document.getElementById(resultId).textContent = commentSubmitted;
            document.getElementById(commentId).value = '';
            fetchComments();
        } else if (response.status === 403) {
            console.log('Forbidden - 403');
            document.getElementById(resultId).textContent = commentRejected;
        } else {
            console.log('Status ' + response.status);
        }
    })
    .catch(error => {
        enableSubmit();
        console.error('Error:', error);
        document.getElementById(resultId).textContent = 'Error submitting comment: ' + error;
    });
}

// Async function to fetch comments from an API
async function fetchComments() {
    const container = document.getElementById(displayCommentsId);
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

// Function to display the comments in the DOM
function displayComments(comments) {
    const container = document.getElementById(displayCommentsId);
    container.innerHTML = '';

    const headerElement = document.createElement('h3');
    headerElement.innerHTML = `Recent Comments`;
    container.appendChild(headerElement);

    for (let i = 0; i < comments.length; i++) {
        const comment = comments[i];

        const commentElement = document.createElement('div');
        commentElement.classList.add('comment');

        const pElement = document.createElement('p');

        const authorElement = document.createElement('span');
        authorElement.classList.add('author');
        authorElement.textContent = comment.author;
        pElement.appendChild(authorElement);
        pElement.appendChild(document.createTextNode(' '));

        const dateElement = document.createElement('span');
        dateElement.classList.add('datetime');
        dateElement.textContent = comment.date;
        pElement.appendChild(dateElement);
        pElement.appendChild(document.createTextNode(' '));

        const contentElement = document.createTextNode(comment.content);
        pElement.appendChild(contentElement);

        commentElement.appendChild(pElement);
        container.appendChild(commentElement);
    }
}

function startup() {
    // Add comment form on page startup.
    // Adding with javascript means the form will not
    // appear if the user has javascript disabled,
    // because we want them to click the link to
    // go to comments.endgameviable.com instead.
    addForm();
    document.getElementById('spiritriot-form').addEventListener('submit', handleSubmit);
    presetForm();
 
    fetchComments();
}

document.addEventListener('DOMContentLoaded', startup);
