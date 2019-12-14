# sudo aptitude install -y python-flask python-mysqldb python-routes
from __future__ import with_statement

try:
    import MySQLdb
    from MySQLdb.cursors import DictCursor
except ImportError:
    import pymysql as MySQLdb
    from pymysql.cursors import DictCursor

from flask import (
        Flask, request, redirect,
        render_template, _app_ctx_stack, Response
        )

import googlecloudprofiler
import json, os

config = {}

app = Flask(__name__, static_url_path='')

def load_config():
    global config
    print "Loading configuration"
    env = os.environ.get('ISUCON_ENV') or 'local'
    with open('../config/common.' + env + '.json') as fp:
        config = json.load(fp)

def connect_db():
    global config
    host = config['database']['host']
    port = config['database']['port']
    username = config['database']['username']
    password = config['database']['password']
    dbname   = config['database']['dbname']
    db = MySQLdb.connect(host=host, port=port, db=dbname, user=username, passwd=password, cursorclass=DictCursor, charset="utf8")
    return db

def init_db():
    print "Initializing database"
    with connect_db() as cur:
        with open('../config/database/initial_data.sql') as fp:
            for line in fp:
                line = line.strip()
                if line:
                    cur.execute(line)

def get_recent_sold():
    cur = get_db().cursor()
    cur.execute('''SELECT stock.seat_id, variation.name AS v_name, ticket.name AS t_name, artist.name AS a_name FROM stock
        JOIN variation ON stock.variation_id = variation.id
        JOIN ticket ON variation.ticket_id = ticket.id
        JOIN artist ON ticket.artist_id = artist.id
        WHERE order_id IS NOT NULL
        ORDER BY order_id DESC LIMIT 10''')
    recent_sold = cur.fetchall()
    cur.close()
    return recent_sold


def get_db():
    top = _app_ctx_stack.top
    if not hasattr(top, 'db'):
        top.db = connect_db()
    return top.db


@app.teardown_appcontext
def close_db_connection(exception):
    top = _app_ctx_stack.top
    if hasattr(top, 'db'):
        top.db.close()

def get_artists(cache=None):
    if not cache:
        cur = get_db().cursor()
        cur.execute('SELECT * FROM artist')
        cache = cur.fetchall()
        cur.close()
    return cache

@app.route("/")
def top_page():
    artists = get_artists()
    recent_sold = get_recent_sold()
    return render_template('index.html', artists=artists, recent_sold=recent_sold)

@app.route("/artist/<int:artist_id>")
def artist_page(artist_id):
    cur = get_db().cursor()

    cur.execute('SELECT id, name FROM artist WHERE id = %s LIMIT 1', (artist_id,))
    artist = cur.fetchone()

    cur.execute('SELECT id, name FROM ticket WHERE artist_id = %s', (artist_id,))
    tickets = cur.fetchall()

    for ticket in tickets:
        cur.execute(
            '''SELECT sum(4096 - sold_count) as cnt FROM variation
                WHERE variation.ticket_id = %s''',
            (ticket['id'],)
        )
        ticket['count'] = cur.fetchone()['cnt']

    cur.close()

    return render_template(
        'artist.html',
        artist=artist,
        tickets=tickets,
        recent_sold=get_recent_sold()
    )


def get_stocks(id, cache={}):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT seat_id, null as order_id FROM stock WHERE variation_id = %s order by id',
            (id,)
        )
        cache[id] = cur.fetchall()
        cur.close()
    return cache[id]

def get_ticket(ticket_id, cache={}):
    if not id in cache:
        cur = get_db().cursor()
        cur.execute(
            'SELECT t.*, a.name AS artist_name FROM ticket t INNER JOIN artist a ON t.artist_id = a.id WHERE t.id = %s LIMIT 1',
            (ticket_id,)
        )
        cache[id] = cur.fetchone()
        cur.close()
    return cache[id]


@app.route("/ticket/<int:ticket_id>")
def ticket_page(ticket_id):
    cur = get_db().cursor()
   
    ticket = get_ticket(ticket_id)

    cur.execute(
        'SELECT id, name, sold_count FROM variation WHERE ticket_id = %s',
        (ticket_id,)
    )
    variations = cur.fetchall()

    for variation in variations:
        stocks = get_stocks(variation['id'])
        variation['vacancy'] = len(stocks) - variation['sold_count']
        variation['stock'] = {}
        i = 0
        for stock in stocks:
            variation['stock'][stock['seat_id']] = None if i >= variation['sold_count'] else "xxx"
            i += 1

    return render_template(
        'ticket.html',
        ticket=ticket,
        variations=variations,
        recent_sold=get_recent_sold()
    )

@app.route("/buy", methods=['POST'])
def buy_page():
    variation_id = int(request.values['variation_id'])
    member_id = request.values['member_id']

    db = get_db()
    cur = db.cursor()
    cur.execute(
        'INSERT INTO order_request (member_id) VALUES (%s)',
        (member_id,)
    )
    order_id = db.insert_id()
    rows = cur.execute(
        'UPDATE stock SET order_id = %s WHERE variation_id = %s AND order_id IS NULL ORDER BY id LIMIT 1',
        (order_id, variation_id)
    )
    cur.execute(
        'UPDATE variation SET sold_count = sold_count + 1 WHERE id = %s',
        (variation_id,)
    )
    if rows > 0:
        cur.execute(
            'SELECT seat_id FROM stock WHERE order_id = %s LIMIT 1',
            (order_id,)
        );
        stock = cur.fetchone()
        db.commit()
        return render_template('complete.html', seat_id=stock['seat_id'], member_id=member_id)
    else:
        db.rollback()
        return render_template('soldout.html')

@app.route("/admin", methods=['GET', 'POST'])
def admin_page():
    if request.method == 'POST':
        init_db()
        return redirect("/admin")
    else:
        return render_template('admin.html')

@app.route("/admin/order.csv")
def admin_csv():
    cur = get_db().cursor()
    cur.execute('''SELECT order_request.*, stock.seat_id, stock.variation_id, stock.updated_at
         FROM order_request JOIN stock ON order_request.id = stock.order_id
         ORDER BY order_request.id ASC''')
    orders = cur.fetchall()
    cur.close()

    body = ''
    for order in orders:
        body += ','.join([str(order['id']), order['member_id'], order['seat_id'], str(order['variation_id']), order['updated_at'].strftime('%Y-%m-%d %X')])
        body += "\n"
    return Response(body, content_type="text/csv")

if __name__ == "__main__":
    load_config()
    port = int(os.environ.get("PORT", '5000'))
    app.run(debug=1, host='0.0.0.0', port=port)
else:
    """
    googlecloudprofiler.start(
        service='hello-profiler',
        service_version='1.0.1',
        verbose=3,
        # project_id='my-project-id'
    )
    """
    load_config()
