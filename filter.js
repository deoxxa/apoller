var tags = [
  'ambient',
  'vaporwave',
  'glitch',
  'synthpop',
  'synthwave',
];

function filter(a) {
  if (a.Year >= 2016) {
    console.log('[+]', 'year=' + a.Year, a.Name);
    return true;
  }

  for (var i = 0; i < tags.length; i++) {
    if (a.Tags.indexOf(tags[i]) !== -1) {
      console.log('[+]', 'tag=' + tags[i], a.Name);
      return true;
    }
  }

  console.log('[-]', a.Name, ':', a.Year, ':', a.Tags.join(', '));

  return false;
}
