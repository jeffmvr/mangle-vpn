import Vue from 'vue'


// error returns the first element in the given array.
Vue.filter('error', (value) => {
  if (Array.isArray(value)) {
    return value[0];
  }
});

// remoteIP returns the IP address portion of an OpenVPN remate address.
Vue.filter('remoteIP', (value) => {
  return value.split(':')[0];
});

// prettyDate returns a nicely formatted date string from a timestamp; this is
// the date only.
Vue.filter('prettyDate', value => {
  return formattedDate(new Date(value));
});

// secsToHoursMins returns the given number of seconds into a string that shows
// the number of minutes and hours.
// Example: 1h 33m
Vue.filter('secsToHoursMins', value => {
    if (value < 60) {
      return `${Math.floor(value)}s`;
    }

    let hours = Math.floor(value / 3600);
    let mins = Math.floor((value - hours * 3600) / 60);

    if (hours > 0) {
      if (mins < 10) {
        return `${hours}h 0${mins}m`;
      } else {
        return `${hours}h ${mins}m`
      }
    }
    return `${mins}m`;
});

// prettyDateTime returns a nicely formatted date and time string from a
// timestamp.
Vue.filter('prettyDateTime', value => {
  let date = new Date(value);
  let dateStr = formattedDate(date);
  let timeStr = formattedTime(date);
  let val = `${dateStr} ${timeStr}`;

  if (val === "12/31/1969 16:00:00") {
    return ""
  }

  return val
});

// firewallRuleAny returns a string representing an 'any' value.
Vue.filter('firewallRuleAny', value => {
  if (value === "" || value === null || value === "all" ) {
    return "All";
  }
  return value;
});

// formattedDate returns a nicely formatted date string for a timestamp.
function formattedDate(date) {
  let month = date.getMonth() + 1;
  let day = date.getDate();
  let year = date.getFullYear();

  if (month < 10) {
    month = `0${month}`;
  }
  if (day < 10) {
    day = `0${day}`;
  }
  return `${month}/${day}/${year}`;
}

// formattedTime returns a string that incldues a nicely formatted time string.
function formattedTime(date) {
  let hours = date.getHours();
  let minutes = date.getMinutes();
  let seconds = date.getSeconds();

  if (hours < 10) {
    hours = `0${hours}`;
  }
  if (minutes < 10) {
    minutes = `0${minutes}`;
  }
  if (seconds < 10) {
    seconds = `0${seconds}`;
  }
  return `${hours}:${minutes}:${seconds}`;
}
